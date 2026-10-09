package main

import (
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"

	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/service"
)

func main() {
	store := mock.NewReviewStore()
	reviews := service.NewReviews(store)
	workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := connectapi.Handler(reviews, workflow, mock.SeedReview, connect.WithInterceptors(connectapi.Logging(logger)))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /runtime", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"mock"}`))
	})
	mux.Handle("/", handler)
	port := os.Getenv("STATECRAFT_PORT")
	if port == "" {
		port = "8081"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	logger.Info("server.started", "address", address, "mode", "mock")
	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
