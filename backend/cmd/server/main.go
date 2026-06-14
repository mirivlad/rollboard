package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"

	"rollboard/internal/httpapi"
	"rollboard/internal/storage/sqlite"
)

func main() {
	defaultAddr := os.Getenv("ROLLBOARD_ADDR")
	if defaultAddr == "" {
		defaultAddr = "127.0.0.1:8080"
	}
	defaultDSN := os.Getenv("ROLLBOARD_DB_PATH")
	if defaultDSN == "" {
		defaultDSN = "./data/rollboard.db"
	}

	addr := flag.String("addr", defaultAddr, "server address")
	dsn := flag.String("dsn", defaultDSN, "sqlite DSN")
	flag.Parse()

	store, err := sqlite.New(*dsn)
	if err != nil {
		log.Fatalf("failed to open storage: %v", err)
	}
	defer store.Close()

	mux := http.NewServeMux()

	api := httpapi.New(store)
	api.RegisterRoutes(mux)

	handler := recoveryMiddleware(loggerMiddleware(corsMiddleware(mux)))

	log.Printf("rollboard server starting on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(200)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(lrw, r)
		log.Printf("%s %s %d", r.Method, r.URL.Path, lrw.statusCode)
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

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %s %s: %v\n%s", r.Method, r.URL.Path, rec, string(debug.Stack()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(500)
				fmt.Fprint(w, `{"error":"internal server error","details":"panic recovered; see backend logs"}`)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
