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
	port := os.Getenv("STATECRAFT_PORT")
	if port == "" {
		port = "8081"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	logger.Info("server.started", "address", address, "mode", "mock")
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
