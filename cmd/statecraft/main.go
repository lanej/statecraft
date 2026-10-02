package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/mock"
	"github.com/lanej/statecraft/internal/service"
)

func main() {
	store := mock.NewReviewStore()
	reviews := service.NewReviews(store)
	workflow := service.NewDemoWorkflow(store, mock.Planner{}, mock.Policies{}, mock.Policies{}, time.Now)

	handler := connectapi.Handler(reviews, workflow, mock.SeedReview)
	port := os.Getenv("STATECRAFT_PORT")
	if port == "" {
		port = "8081"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	log.Printf("Statecraft mock API listening on http://%s (no live infrastructure)", address)
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}
