package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LeifistLive/zimacube-fan-controller/internal/store"
	"github.com/LeifistLive/zimacube-fan-controller/internal/webui"
)

const (
	sessionCookieName = "zimafan_session"
	sessionTTL        = 24 * time.Hour
	loginRateInterval = 2 * time.Second
	bcryptCost        = 12
	// sweepInterval controls how often expired sessions and stale per-IP
	// login rate gates are dropped from memory, see sweepExpiredPeriodically.
	sweepInterval = time.Hour
	// staleLoginLimiter: a per-IP gate that has not been used in this long is
	// safe to forget; it will simply be recreated on the next attempt.
	staleLoginLimiter = 24 * time.Hour
)

// auth holds the admin credential (hashed) and the in-memory session store
// that gates the whole dashboard. A disabled auth (enabled == false) treats
// every request as authenticated; that is the deliberate fallback when
// ADMIN_PASSWORD is unset, so a freshly deployed container is not locked out
// before it has been configured, mirroring how the old, now-removed
// API_TOKEN behaved when left empty.
type auth struct {
	user         string
	passwordHash []byte
	enabled      bool

	mu       sync.Mutex
	sessions map[string]time.Time

	// loginLimiters rate-limits login attempts per client IP (never per
	// forwarded/spoofable header) so one abusive or mistaken client cannot
	// lock the real admin out of their own login by exhausting a single
	// shared gate.
	loginLimitersMu sync.Mutex
	loginLimiters   map[string]*rateGate
}

// newAuth returns an error when ADMIN_PASSWORD is set but unusable (bcrypt
// rejects anything over 72 bytes). That must never be treated as "leave
// auth disabled": disabled means the whole dashboard is open, which is the
// opposite of what a configuration error should do. The caller (New) turns
// this into a startup failure instead, so a broken password blocks the
// service from ever serving a request rather than silently exposing it.
func newAuth(user, password string) (*auth, error) {
	a := &auth{
		user:          user,
		sessions:      map[string]time.Time{},
		loginLimiters: map[string]*rateGate{},
	}
	go a.sweepExpiredPeriodically()

	if password == "" {
		log.Printf("[WARN] ADMIN_PASSWORD is empty, the dashboard is reachable without login")
		return a, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("ADMIN_PASSWORD could not be hashed: %w", err)
	}
	a.passwordHash = hash
	a.enabled = true
	return a, nil
}

// verify always runs bcrypt, even for a wrong username, so a mismatched
// username does not respond measurably faster than a wrong password.
func (a *auth) verify(user, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) == 1
	passwordOK := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password)) == nil
	return userOK && passwordOK
}

func newSessionID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *auth) createSession() (string, error) {
	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.sessions[id] = time.Now().Add(sessionTTL)
	a.mu.Unlock()
	return id, nil
}

// validSession reports whether id is a live session. A hit slides the
// expiry forward, so an active user is never logged out mid-session.
func (a *auth) validSession(id string) bool {
	if id == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	expiry, ok := a.sessions[id]
	if !ok || time.Now().After(expiry) {
		delete(a.sessions, id)
		return false
	}
	a.sessions[id] = time.Now().Add(sessionTTL)
	return true
}

func (a *auth) destroySession(id string) {
	a.mu.Lock()
	delete(a.sessions, id)
	a.mu.Unlock()
}

// loginLimiterFor returns the rate gate for a client IP, creating one on
// first use.
func (a *auth) loginLimiterFor(ip string) *rateGate {
	a.loginLimitersMu.Lock()
	defer a.loginLimitersMu.Unlock()
	gate, ok := a.loginLimiters[ip]
	if !ok {
		gate = &rateGate{}
		a.loginLimiters[ip] = gate
	}
	return gate
}

// sweepExpiredPeriodically runs for the lifetime of the process: sessions
// are otherwise only ever removed lazily when someone tries to use that
// exact (by then expired) cookie again, so an abandoned session (browser
// closed, cookie cleared) would sit in memory until the process restarts.
// The same applies to per-IP login rate gates for clients that stop trying.
func (a *auth) sweepExpiredPeriodically() {
	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()
	for range ticker.C {
		a.sweepExpired()
	}
}

func (a *auth) sweepExpired() {
	now := time.Now()

	a.mu.Lock()
	for id, expiry := range a.sessions {
		if now.After(expiry) {
			delete(a.sessions, id)
		}
	}
	a.mu.Unlock()

	a.loginLimitersMu.Lock()
	for ip, gate := range a.loginLimiters {
		gate.mu.Lock()
		// A zero next means Allow was never called on this gate yet (it was
		// just created); that is not the same as stale and must not be swept
		// on the very next tick.
		stale := !gate.next.IsZero() && now.Sub(gate.next) > staleLoginLimiter
		gate.mu.Unlock()
		if stale {
			delete(a.loginLimiters, ip)
		}
	}
	a.loginLimitersMu.Unlock()
}

func (a *App) authenticated(r *http.Request) bool {
	if !a.auth.enabled {
		return true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	return a.auth.validSession(cookie.Value)
}

// requestIsSecure reports whether r arrived over TLS, either terminated by
// this process directly or by a reverse proxy in front of it. Only the
// standard X-Forwarded-Proto header is trusted; there is no configuration
// for "trusted proxy" here, but this header is only ever consulted to
// decide the cookie's Secure flag, never for anything access-control
// relevant, so a client forging it can at most make its own cookie
// non-Secure (or Secure when plain HTTP still works fine on localhost).
func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

// clientIP extracts the TCP peer address, deliberately ignoring
// X-Forwarded-For: unlike the Secure-cookie heuristic above, this value
// gates the login rate limiter, and trusting a client-supplied header there
// would let an attacker either evade the limit (a different value per
// request) or lock out the real admin (submit the admin's own address).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// requireAuth protects every route except GET/POST /login and GET
// /api/health (kept open for the Docker healthcheck and external
// monitoring). It also enforces checkSameOrigin, since cookie based
// sessions make CSRF relevant for reads too, not just the writes guardWrite
// already protected.
func (a *App) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checkSameOrigin(r); err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if a.authenticated(r) {
			next(w, r)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
}

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if a.authenticated(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	serveAsset(w, "text/html; charset=utf-8", webui.LoginHTML)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := checkSameOrigin(r); err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if !a.auth.loginLimiterFor(clientIP(r)).Allow(time.Now(), loginRateInterval) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, please wait a moment"})
		return
	}
	if !a.auth.enabled {
		writeJSON(w, http.StatusOK, map[string]string{"status": "auth disabled"})
		return
	}

	user, password, ok := r.BasicAuth()
	if !ok || !a.auth.verify(user, password) {
		a.appendEvent(store.Event{
			Time:    time.Now(),
			Type:    "login-failed",
			Message: fmt.Sprintf("failed login attempt for %q from %s", user, clientIP(r)),
		})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}

	session, err := a.auth.createSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create session"})
		return
	}
	http.SetCookie(w, sessionCookie(r, session, int(sessionTTL.Seconds())))
	a.appendEvent(store.Event{
		Time:    time.Now(),
		Type:    "login",
		Message: fmt.Sprintf("login succeeded from %s", clientIP(r)),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.auth.destroySession(cookie.Value)
	}
	http.SetCookie(w, sessionCookie(r, "", -1))
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}
