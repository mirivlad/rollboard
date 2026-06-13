package main

import (
	"flag"
	"log"
	"net/http"
	"os"

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

	handler := corsMiddleware(mux)

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
