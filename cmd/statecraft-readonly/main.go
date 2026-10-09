// This executable has no mock store, planner, executor, or mutation handlers.
package main

import (
	"flag"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/githubread"
	"github.com/lanej/statecraft/internal/adapters/readruntime"
)

func main() {
	local := flag.Bool("local", false, "serve on loopback without IAP for local read-only development")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	audience := os.Getenv("STATECRAFT_IAP_AUDIENCE")
	if *local && os.Getenv("K_SERVICE") != "" {
		log.Fatal("local mode is forbidden on Cloud Run")
	}
	if !*local && audience == "" {
		log.Fatal("STATECRAFT_IAP_AUDIENCE is required")
	}
	reader, err := githubread.New(os.Getenv("STATECRAFT_GITHUB_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	dist := os.Getenv("STATECRAFT_WEB_DIR")
	if dist == "" {
		dist = "web/dist"
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		log.Fatal("build web/ before starting the read-only server")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	host := "0.0.0.0"
	if *local {
		host = "127.0.0.1"
	}
	server := &http.Server{
		Addr: net.JoinHostPort(host, port), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 * 1024,
		Handler: readruntime.Handler(reader, os.DirFS(dist), audience, *local, connect.WithInterceptors(connectapi.Logging(logger))),
	}
	logger.Info("server.started", "address", server.Addr, "mode", "read_only", "iap", !*local)
	log.Fatal(server.ListenAndServe())
}
