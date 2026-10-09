package readruntime

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestProductionProtectsEveryDataSurface(t *testing.T) {
	handler := Handler(nil, fstest.MapFS{"index.html": {Data: []byte("web")}}, "expected", false)
	for _, path := range []string{"/", "/runtime", "/assets/app.js", "/statecraft.v1.SourceEvidenceService/ListChanges", "/statecraft.v1.DemoWorkflowService/ActOnDemo"} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Goog-Authenticated-User-Email", "admin@easypost.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("%s exposed without a signed assertion: %d", path, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s response may be cached", path)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("health probe must contain no data")
	}
}

func TestReadOnlyCompositionHasNoDemoMutations(t *testing.T) {
	handler := Handler(nil, fstest.MapFS{"index.html": {Data: []byte("web")}}, "", true)
	for _, path := range []string{"/statecraft.v1.DemoWorkflowService/CreateDemo", "/statecraft.v1.DemoWorkflowService/ActOnDemo"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 404 {
			t.Errorf("demo route mounted at %s: %d", path, w.Code)
		}
	}
}
