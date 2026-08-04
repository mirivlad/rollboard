package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"rollboard/internal/catalog"
	"rollboard/internal/config"
	"rollboard/internal/httpapi"
	"rollboard/internal/identity"
	"rollboard/internal/realtime"
	"rollboard/internal/room"
	"rollboard/internal/storage/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	addr := flag.String("addr", cfg.Addr, "server address")
	flag.Parse()

	store, err := postgres.New(context.Background(), cfg.DatabaseURL, cfg.DatabaseMaxConns)
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
	roomService, err := room.NewService(store.Pool(), catalogService)
	if err != nil {
		log.Fatalf("failed to create room service: %v", err)
	}
	realtimeBackplane, err := realtime.NewRedisBackplane(context.Background(), cfg.RedisURL)
	if err != nil {
		log.Fatalf("failed to connect realtime backplane: %v", err)
	}
	realtimeHub, err := realtime.NewHub(roomService, realtime.WithBackplane(realtimeBackplane))
	if err != nil {
		_ = realtimeBackplane.Close()
		log.Fatalf("failed to create realtime hub: %v", err)
	}
	defer realtimeHub.Close()
	appOrigin, err := url.Parse(cfg.AppOrigin)
	if err != nil {
		log.Fatalf("invalid application origin: %v", err)
	}
	api := httpapi.New(store).
		WithIdentity(identityRepository).
		WithCatalog(catalogService).
		WithRooms(roomService).
		WithRealtimeHub(realtimeHub).
		WithAuthOptions(httpapi.AuthOptions{
			CookieSecure: cfg.CookieSecure, SessionTTL: cfg.SessionTTL,
			WebSocketOriginPatterns: []string{appOrigin.Host},
			RateLimit:               cfg.AuthRateLimit,
			TrustedProxies:          cfg.TrustedProxies,
		}).
		WithLocales(httpapi.LocaleOptions{Dir: cfg.LocalesDir}).
		WithUploads(httpapi.UploadOptions{
			Dir:             cfg.UploadsDir,
			Records:         postgres.NewUploadRecords(store.Pool()),
			PerAccountBytes: cfg.UploadQuotaBytes,
			TotalBytes:      cfg.UploadTotalBytes,
			RatePerMinute:   cfg.UploadRateLimit,
		})
	api.RegisterRoutes(mux)

	// A crash between writing an image and recording it leaves a file nothing
	// references, and nothing else would ever remove it.
	if removed, err := api.SweepOrphanedUploads(context.Background()); err != nil {
		log.Printf("could not sweep orphaned uploads: %v", err)
	} else if removed > 0 {
		log.Printf("removed %d orphaned upload(s)", removed)
	}
	if cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(cfg.StaticDir))
	}

	handler := recoveryMiddleware(loggerMiddleware(corsMiddleware(mux, cfg.AppOrigin)))

	// Timeouts, chosen for what this server actually serves.
	//
	// ReadHeaderTimeout is short because headers arrive in one go; a
	// connection that dribbles them out is holding a slot for nothing.
	// ReadTimeout has to allow a 2 MB image over a slow link. There is
	// deliberately no WriteTimeout: it applies to a WebSocket for the whole
	// life of the connection, so setting one would cut every live room off at
	// the deadline. Idle connections are bounded instead.
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Shut down in the order things depend on each other: stop accepting, let
	// requests in flight finish, then close the hub and its backplane, and only
	// then the database the hub was writing to.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("rollboard server starting on %s", *addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-shutdown:
		log.Printf("received %s, shutting down", sig)
		// Bounded, because a container runtime will send SIGKILL anyway; this
		// decides how much of that grace period requests get.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown timed out, closing: %v", err)
			_ = server.Close()
		}
		log.Print("shutdown complete")
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

// Hijack preserves WebSocket upgrade support through the logging wrapper.
func (lrw *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := lrw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
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
