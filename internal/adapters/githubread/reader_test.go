package githubread

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lanej/statecraft/internal/domain"
)

func fixtureReader(t *testing.T, alter func(http.ResponseWriter, *http.Request, any) any) *Reader {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.URL.Path, "/repos/easypost/platform-infra/") {
			t.Errorf("repository escaped: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		var payload any
		switch r.URL.Path {
		case "/repos/easypost/platform-infra/pulls":
			payload = []any{map[string]any{"number": 12, "title": "Real source", "user": nil, "head": map[string]string{"sha": "abc"}, "base": map[string]string{"ref": "main"}}}
		case "/repos/easypost/platform-infra/pulls/12":
			payload = map[string]any{"number": 12, "title": "Real source", "head": map[string]string{"sha": "abc"}, "base": map[string]string{"sha": "base", "ref": "main"}, "changed_files": 1, "state": "open"}
		case "/repos/easypost/platform-infra/pulls/12/files":
			payload = []any{map[string]any{"filename": "live/test.tf", "status": "modified", "additions": 1, "deletions": 2, "patch": "<script>untrusted</script>"}}
		case "/repos/easypost/platform-infra/pulls/12/reviews":
			payload = []any{
				map[string]any{"id": 1, "state": "APPROVED", "commit_id": "old", "submitted_at": "2026-10-09T12:00:00Z", "user": nil},
				map[string]any{"id": 2, "state": "PENDING"},
			}
		case "/repos/easypost/platform-infra/commits/abc/check-runs":
			payload = map[string]any{"total_count": 1, "check_runs": []any{map[string]any{"name": "Atlantis", "head_sha": "abc", "status": "completed", "conclusion": "failure"}}}
		case "/repos/easypost/platform-infra/commits/abc/status":
			payload = map[string]any{"sha": "abc", "total_count": 1, "statuses": []any{map[string]any{"context": "atlantis/plan", "state": "success"}}}
		default:
			t.Errorf("unexpected provider path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if alter != nil {
			payload = alter(w, r, payload)
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(server.Close)
	reader, err := newReader(server.URL, "test-token", domain.RepositoryRef{Owner: "easypost", Name: "platform-infra"})
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func TestCaptureKeepsSourceEvidenceSeparate(t *testing.T) {
	reader := fixtureReader(t, nil)
	list, err := reader.ListChanges(context.Background())
	if err != nil || len(list.Changes) != 1 || list.Changes[0].Author != "" {
		t.Fatalf("list: %+v, %v", list, err)
	}
	evidence, err := reader.GetSourceEvidence(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Files) != 1 || !evidence.Files[0].PatchAvailable {
		t.Fatalf("files: %+v", evidence.Files)
	}
	if len(evidence.Reviews) != 1 || evidence.Reviews[0].PlanSetID != "" || evidence.Reviews[0].CommitSHA != "old" {
		t.Fatalf("reviews: %+v", evidence.Reviews)
	}
	if len(evidence.Checks) != 2 || evidence.Checks[0].Conclusion != "failure" || evidence.Checks[1].EvidenceSource != "GitHub commit status" {
		t.Fatalf("checks: %+v", evidence.Checks)
	}
}

func TestRejectsChangingSourceOrMissingFiles(t *testing.T) {
	for _, kind := range []string{"head", "base", "files", "checks", "status"} {
		t.Run(kind, func(t *testing.T) {
			reads := 0
			reader := fixtureReader(t, func(_ http.ResponseWriter, r *http.Request, payload any) any {
				if r.URL.Path == "/repos/easypost/platform-infra/pulls/12" {
					reads++
					if reads > 1 && (kind == "head" || kind == "base") {
						payload.(map[string]any)[kind] = map[string]string{"sha": "new"}
					}
				}
				if kind == "files" && strings.HasSuffix(r.URL.Path, "/files") {
					return []any{}
				}
				if kind == "checks" && strings.HasSuffix(r.URL.Path, "/check-runs") {
					payload.(map[string]any)["check_runs"] = []any{map[string]any{"head_sha": "wrong"}}
				}
				if kind == "status" && strings.HasSuffix(r.URL.Path, "/status") {
					payload.(map[string]any)["sha"] = "wrong"
				}
				return payload
			})
			_, err := reader.GetSourceEvidence(context.Background(), 12)
			if !errors.Is(err, ErrChanged) {
				t.Fatalf("got %v, want changed-source rejection", err)
			}
		})
	}
}

func TestReadTransportRejectsMutation(t *testing.T) {
	req, _ := http.NewRequest("POST", "https://api.github.com/repos/easypost/platform-infra/pulls", nil)
	_, err := (readTransport{}).RoundTrip(req)
	if err == nil {
		t.Fatal("provider mutation was permitted")
	}
}

func TestProviderFailureDoesNotExposeBody(t *testing.T) {
	reader := fixtureReader(t, func(_ http.ResponseWriter, _ *http.Request, _ any) any {
		return map[string]string{"message": "private diagnostic"}
	})
	_, err := reader.ListChanges(context.Background())
	if !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "private diagnostic") {
		t.Fatalf("error: %v", err)
	}
}

func TestPaginationAndUnavailablePatch(t *testing.T) {
	reader := fixtureReader(t, func(w http.ResponseWriter, r *http.Request, payload any) any {
		if strings.HasSuffix(r.URL.Path, "/pulls/12") {
			payload.(map[string]any)["changed_files"] = 2
		}
		if strings.HasSuffix(r.URL.Path, "/files") || strings.HasSuffix(r.URL.Path, "/reviews") {
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `<https://api.github.com/page2>; rel="next"`)
			}
			if r.URL.Query().Get("page") == "2" {
				if strings.HasSuffix(r.URL.Path, "/files") {
					return []any{map[string]any{"filename": "binary.dat"}}
				}
				return []any{map[string]any{"state": "DISMISSED", "submitted_at": "2026-10-09T12:00:00Z"}}
			}
		}
		return payload
	})
	evidence, err := reader.GetSourceEvidence(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.Files) != 2 || evidence.Files[1].PatchAvailable || len(evidence.Reviews) != 2 {
		t.Fatalf("capture: %+v", evidence)
	}
}

func TestCaptureLimitsFailClosed(t *testing.T) {
	for _, limit := range []string{"pages", "bytes", "missing checks", "missing statuses"} {
		t.Run(limit, func(t *testing.T) {
			reader := fixtureReader(t, func(w http.ResponseWriter, r *http.Request, payload any) any {
				if strings.HasSuffix(r.URL.Path, "/files") {
					if limit == "pages" {
						w.Header().Set("Link", `<https://api.github.com/next>; rel="next"`)
					}
					if limit == "bytes" {
						payload.([]any)[0].(map[string]any)["patch"] = strings.Repeat("x", 4*1024*1024+1)
					}
				}
				if limit == "missing checks" && strings.HasSuffix(r.URL.Path, "/check-runs") {
					return map[string]int{"total_count": 0}
				}
				if limit == "missing statuses" && strings.HasSuffix(r.URL.Path, "/status") {
					return map[string]any{"sha": "abc", "total_count": 0}
				}
				return payload
			})
			_, err := reader.GetSourceEvidence(context.Background(), 12)
			want := ErrIncomplete
			if limit == "missing checks" || limit == "missing statuses" {
				want = ErrUnavailable
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
		})
	}
}

func TestProviderAuthorizationAndRateLimitFailures(t *testing.T) {
	for _, status := range []int{401, 403, 429} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			reader := fixtureReader(t, func(w http.ResponseWriter, _ *http.Request, _ any) any {
				w.WriteHeader(status)
				return map[string]string{"message": "private provider diagnostic"}
			})
			_, err := reader.ListChanges(context.Background())
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("provider failure: %v", err)
			}
		})
	}
}

func TestRecentListDisclosesTruncation(t *testing.T) {
	reader := fixtureReader(t, func(w http.ResponseWriter, _ *http.Request, payload any) any {
		w.Header().Set("Link", `<https://api.github.com/next>; rel="next"`)
		return payload
	})
	list, err := reader.ListChanges(context.Background())
	if err != nil || !list.Truncated {
		t.Fatalf("list: %+v, %v", list, err)
	}
}
