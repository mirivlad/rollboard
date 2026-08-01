package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"

	"rollboard/internal/catalog"
	"rollboard/internal/config"
	"rollboard/internal/httpapi"
	"rollboard/internal/identity"
	"rollboard/internal/storage/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	addr := flag.String("addr", cfg.Addr, "server address")
	flag.Parse()

	store, err := postgres.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	mux := http.NewServeMux()

	identityRepository, err := identity.NewRepository(store.Pool())
	if err != nil {
		log.Fatalf("failed to create identity repository: %v", err)
	}
	catalogService, err := catalog.NewService(store.Pool())
	if err != nil {
		log.Fatalf("failed to create catalog service: %v", err)
	}
	api := httpapi.New(store).
		WithIdentity(identityRepository).
		WithCatalog(catalogService).
		WithAuthOptions(httpapi.AuthOptions{CookieSecure: cfg.CookieSecure, SessionTTL: cfg.SessionTTL})
	api.RegisterRoutes(mux)
	if cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(cfg.StaticDir))
	}

	handler := recoveryMiddleware(loggerMiddleware(corsMiddleware(mux, cfg.AppOrigin)))

	log.Printf("rollboard server starting on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func spaHandler(staticDir string) http.Handler {
	files := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(requestedPath); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}

func corsMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		log.Printf("request_id=%s method=%s path=%s status=%d", requestID, r.Method, r.URL.Path, lrw.statusCode)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newRequestID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(bytes)
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %s %s: %v\n%s", r.Method, r.URL.Path, rec, string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, `{"error":"internal server error","details":"request failed unexpectedly","code":"INTERNAL_ERROR"}`)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
