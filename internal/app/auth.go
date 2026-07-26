package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/LeifistLive/zimacube-fan-controller/internal/webui"
)

const (
	sessionCookieName = "zimafan_session"
	sessionTTL        = 24 * time.Hour
	loginRateInterval = 2 * time.Second
	bcryptCost        = 12
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

	loginLimiter rateGate
}

func newAuth(user, password string) *auth {
	a := &auth{user: user, sessions: map[string]time.Time{}}
	if password == "" {
		log.Printf("[WARN] ADMIN_PASSWORD ist leer, das Dashboard ist ohne Login erreichbar")
		return a
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		// GenerateFromPassword only fails for a password over 72 bytes; fail
		// closed (auth stays disabled, loudly) rather than half-configured.
		log.Printf("[ERROR] ADMIN_PASSWORD konnte nicht gehasht werden (%v), Login bleibt deaktiviert", err)
		return a
	}
	a.passwordHash = hash
	a.enabled = true
	return a
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

func sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
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
	if !a.auth.loginLimiter.Allow(time.Now(), loginRateInterval) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "zu viele Loginversuche, bitte kurz warten"})
		return
	}
	if !a.auth.enabled {
		writeJSON(w, http.StatusOK, map[string]string{"status": "auth disabled"})
		return
	}

	user, password, ok := r.BasicAuth()
	if !ok || !a.auth.verify(user, password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Benutzername oder Passwort falsch"})
		return
	}

	session, err := a.auth.createSession()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Session konnte nicht erstellt werden"})
		return
	}
	http.SetCookie(w, sessionCookie(session, int(sessionTTL.Seconds())))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.auth.destroySession(cookie.Value)
	}
	http.SetCookie(w, sessionCookie("", -1))
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}
