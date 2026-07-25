package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/LeifistLive/zimacube-fan-controller/internal/app"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg := app.Config{
		I2CBus:              envInt("I2C_BUS", 0, 0, 63),
		I2CAddress:          env("I2C_ADDRESS", "0x69"),
		I2CTimeout:          envSeconds("I2C_TIMEOUT_SECONDS", 5, 1, 120),
		I2CRetries:          envInt("I2C_RETRIES", 3, 1, 20),
		CheckInterval:       envSeconds("CHECK_INTERVAL_SECONDS", 15, 5, 3600),
		HistoryInterval:     envSeconds("HISTORY_INTERVAL_SECONDS", 300, 5, 86400),
		DetectInterval:      envSeconds("DETECT_INTERVAL_SECONDS", 300, 15, 86400),
		ListenAddress:       env("LISTEN_ADDRESS", ":8080"),
		APIToken:            os.Getenv("API_TOKEN"),
		VarINI:              env("UNRAID_VAR_INI", "/var/local/emhttp/var.ini"),
		DisksINI:            env("UNRAID_DISKS_INI", "/var/local/emhttp/disks.ini"),
		DataDir:             env("DATA_DIR", "/data"),
		MaxLogLines:         envInt("MAX_LOG_LINES", 20000, 100, 1000000),
		SafeShutdownPercent: envInt("SAFE_SHUTDOWN_PERCENT", 0, 0, 100),
	}

	service, err := app.New(cfg)
	if err != nil {
		log.Fatalf("[ERROR] Start fehlgeschlagen: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("[INFO] ZimaCube Fan Controller v%s startet", app.Version)
	log.Printf("[INFO] I²C Bus %d, Adresse %s", cfg.I2CBus, cfg.I2CAddress)
	log.Printf("[INFO] Web/API auf %s", cfg.ListenAddress)

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           service.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// The HTTP server starts before the controller search, so the dashboard and
	// the Docker healthcheck are reachable even when I²C is not.
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] HTTP-Server: %v", err)
			cancel()
		}
	}()

	if err := service.AwaitController(ctx, 60*time.Second); err != nil {
		log.Printf("[WARN] I²C-Controller nicht gefunden (%v), Dienst läuft weiter und versucht es erneut", err)
	} else {
		log.Printf("[OK] I²C-Controller gefunden")
	}

	service.Run(ctx)
	service.ApplySafeState(cfg.SafeShutdownPercent)
	log.Printf("[INFO] Container wird beendet")
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// envInt logs and clamps instead of silently falling back, so a typo in the
// stack definition is visible in the container log.
func envInt(name string, fallback, minimum, maximum int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	number, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("[WARN] %s=%q ist keine Zahl, verwende %d", name, raw, fallback)
		return fallback
	}
	if number < minimum {
		log.Printf("[WARN] %s=%d ist kleiner als %d, verwende %d", name, number, minimum, minimum)
		return minimum
	}
	if number > maximum {
		log.Printf("[WARN] %s=%d ist größer als %d, verwende %d", name, number, maximum, maximum)
		return maximum
	}
	return number
}

func envSeconds(name string, fallback, minimum, maximum int) time.Duration {
	return time.Duration(envInt(name, fallback, minimum, maximum)) * time.Second
}
