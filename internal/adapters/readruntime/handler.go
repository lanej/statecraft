// Package readruntime composes the deployable HTTP surface independently of
// demo workflows and provider mutation capabilities.
package readruntime

import (
	"io/fs"
	"net/http"

	"connectrpc.com/connect"
	"github.com/lanej/statecraft/internal/adapters/connectapi"
	"github.com/lanej/statecraft/internal/adapters/iap"
	"github.com/lanej/statecraft/internal/ports"
)

func Handler(reader ports.SourceEvidenceReader, web fs.FS, audience string, local bool, options ...connect.HandlerOption) http.Handler {
	mux := http.NewServeMux()
	path, handler := connectapi.SourceHandler(reader, options...)
	mux.Handle(path, handler)
	mux.HandleFunc("GET /runtime", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"read_only"}`))
	})
	mux.Handle("/", http.FileServer(http.FS(web)))
	var app http.Handler = mux
	if !local {
		app = iap.Protect(audience, app)
	}
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	outer.Handle("/", app)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		outer.ServeHTTP(w, r)
	})
}
