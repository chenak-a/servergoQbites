package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/chenak/hackernews/database"
	"github.com/chenak/hackernews/graph"
	"github.com/chenak/hackernews/graph/generated"
)

const (
	defaultPort     = "8090"
	defaultMongoURI = "mongodb://localhost:27017"
	// Generous for the UI's candle query (~40 fields) but blocks aliased fan-out.
	maxQueryComplexity = 1000
	readHeaderTimeout  = 5 * time.Second
	writeTimeout       = 30 * time.Second
	idleTimeout        = 120 * time.Second
	shutdownTimeout    = 10 * time.Second
	healthTimeout      = 3 * time.Second
)

// healthcheck probes this process's /healthz; the distroless image has no
// curl, so the compose healthcheck runs the binary as `server healthcheck`.
func healthcheck(port string) int {
	client := &http.Client{Timeout: healthTimeout}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		log.Print(err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthz: %s", resp.Status)
		return 1
	}
	return 0
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// corsMiddleware echoes the request origin only when it is in `allowed`. The
// web UI is served same-origin through the Vite proxy and needs no CORS.
func corsMiddleware(allowed map[string]bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseOrigins(csv string) map[string]bool {
	out := map[string]bool{}
	for _, o := range strings.Split(csv, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out[o] = true
		}
	}
	return out
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck(getenv("PORT", defaultPort)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, disconnect, err := database.Connect(ctx, getenv("MONGO_URI", defaultMongoURI))
	if err != nil {
		log.Fatal(err)
	}

	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: &graph.Resolver{Store: store},
	}))
	srv.Use(extension.FixedComplexityLimit(maxQueryComplexity))

	origins := parseOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	mux := http.NewServeMux()
	mux.Handle("/query", corsMiddleware(origins, srv))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
		defer cancel()
		if err := store.Ping(ctx); err != nil {
			http.Error(w, "mongo unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	if os.Getenv("ENABLE_PLAYGROUND") == "true" {
		mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}

	port := getenv("PORT", defaultPort)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	go func() {
		log.Printf("GraphQL API listening on :%s (/query)", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if err := disconnect(shutdownCtx); err != nil {
		log.Printf("mongo disconnect: %v", err)
	}
}
