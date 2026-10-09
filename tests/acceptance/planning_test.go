package acceptance_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	gh "github.com/google/go-github/v92/github"
	"github.com/lanej/statecraft/internal/adapters/atlantis"
	githubadapter "github.com/lanej/statecraft/internal/adapters/github"
	"github.com/lanej/statecraft/internal/adapters/httpapi"
	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/service"
)

func TestProposedChangePlanningAcceptance(t *testing.T) {
	for _, tc := range []struct {
		name, action, response string
		status                 int
		complete, stale        bool
		attemptStatus          string
		recheckFails           bool
	}{
		{"opened proposal plans", "opened", success, 200, true, false, "succeeded", false},
		{"synchronize plans", "synchronize", success, 200, true, false, "succeeded", false},
		{"failed root retains evidence", "opened", failure, 500, false, false, "failed", false},
		{"missing root remains incomplete", "opened", missing, 200, false, false, "unknown", false},
		{"commit changes during planning", "synchronize", success, 200, false, true, "succeeded", false},
		{"discarded plans remain incomplete", "opened", discarded, 200, false, false, "succeeded", false},
		{"freshness failure retains plan evidence", "opened", success, 200, false, true, "succeeded", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sourceReads, planCalls atomic.Int32
			githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer test-github-token" {
					t.Errorf("unexpected GitHub request: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/repos/acme/infra/pulls/42":
					head := "commit-a"
					if sourceReads.Add(1) > 1 {
						if tc.recheckFails {
							w.WriteHeader(503)
							fmt.Fprint(w, `{"message":"source unavailable"}`)
							return
						}
						if tc.stale {
							head = "commit-b"
						}
					}
					fmt.Fprintf(w, `{"number":42,"title":"Change sample infrastructure","state":"open","user":{"login":"reviewer"},"head":{"sha":%q,"ref":"feature/change"},"base":{"ref":"main"}}`, head)
				case "/repos/acme/infra/pulls/42/files":
					fmt.Fprint(w, `[{"filename":"infra/main.tf","status":"modified","changes":1}]`)
				case "/repos/acme/infra/pulls/42/reviews":
					fmt.Fprint(w, `[]`)
				default:
					t.Errorf("unexpected GitHub path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer githubServer.Close()
			client, err := gh.NewClient(gh.WithHTTPClient(githubServer.Client()), gh.WithURLs(gh.Ptr(githubServer.URL+"/"), nil), gh.WithAuthToken("test-github-token"))
			if err != nil {
				t.Fatal(err)
			}
			atlantisServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				planCalls.Add(1)
				if r.Method != "POST" || r.URL.Path != "/api/plan" || r.Header.Get("X-Atlantis-Token") != "test-atlantis-token" {
					t.Errorf("unexpected Atlantis request: %s %s", r.Method, r.URL.Path)
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				expected := map[string]any{"Repository": "acme/infra", "Ref": "commit-a", "base_branch": "main", "Type": "Github", "PR": float64(42), "Projects": []any{"sample"}}
				if !reflect.DeepEqual(payload, expected) {
					t.Errorf("Atlantis payload = %#v, want %#v", payload, expected)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer atlantisServer.Close()
			planner, err := atlantis.NewClient(atlantisServer.URL, "test-atlantis-token", atlantisServer.Client())
			if err != nil {
				t.Fatal(err)
			}
			workflow := service.NewProposedPlans(githubadapter.NewSourceControl(client), planner)
			ingress := httpapi.NewGitHubWebhook("webhook-secret", workflow, map[string][]domain.RootSelector{
				"acme/infra": {{ID: "root-sample", PlannerRef: "sample", Directory: "infra", Workspace: "default"}},
			})
			payload := fmt.Sprintf(`{"action":%q,"number":42,"repository":{"full_name":"acme/infra"},"pull_request":{"head":{"sha":"untrusted-payload-head"}}}`, tc.action)
			deliver := func(body, delivery, signature, event string) *httptest.ResponseRecorder {
				req := httptest.NewRequest("POST", "/webhooks/github", bytes.NewBufferString(body))
				req.Header.Set("X-GitHub-Event", event)
				req.Header.Set("X-GitHub-Delivery", delivery)
				req.Header.Set("X-Hub-Signature-256", signature)
				recorder := httptest.NewRecorder()
				ingress.ServeHTTP(recorder, req)
				return recorder
			}
			if got := deliver(payload, "invalid", "sha256=00", "pull_request"); got.Code != 401 {
				t.Fatalf("bad signature status = %d", got.Code)
			}
			if planCalls.Load() != 0 || sourceReads.Load() != 0 {
				t.Fatal("invalid signature reached providers")
			}
			if got := deliver(payload, "ignored", sign(payload), "push"); got.Code != 204 {
				t.Fatalf("ignored event status = %d", got.Code)
			}
			response := deliver(payload, "delivery-1", sign(payload), "pull_request")
			expectedStatus := 200
			if tc.recheckFails {
				expectedStatus = 502
			}
			if response.Code != expectedStatus {
				t.Fatalf("webhook status = %d: %s", response.Code, response.Body.String())
			}
			var receipt httpapi.PlanningReceipt
			if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.Result == nil || (tc.recheckFails && receipt.Failure == "") {
				t.Fatalf("receipt lost result/failure: %#v", receipt)
			}
			result := *receipt.Result
			if result.Complete != tc.complete || result.Stale != tc.stale || result.PlannedHeadSHA != "commit-a" {
				t.Fatalf("proposal result = %#v", result)
			}
			if result.Source.Change.Number != 42 || result.Source.Change.Repository.FullName() != "acme/infra" || len(result.Source.Files) != 1 {
				t.Fatalf("source identity = %#v", result.Source)
			}
			if len(result.Run.Attempts) != 1 || result.Run.Attempts[0].RootID != "root-sample" || result.Run.Attempts[0].Status != tc.attemptStatus {
				t.Fatalf("plan evidence = %#v", result.Run)
			}
			if tc.attemptStatus == "succeeded" && result.Run.Attempts[0].Output != "Plan: 1 to add." {
				t.Fatal("plan output lost")
			}
			if tc.attemptStatus == "failed" && result.Run.Attempts[0].Failure != "invalid configuration" {
				t.Fatal("failure evidence lost")
			}
			if got := deliver(payload, "delivery-1", sign(payload), "pull_request"); got.Code != response.Code || got.Body.String() != response.Body.String() {
				t.Fatalf("duplicate did not preserve receipt: %d %s", got.Code, got.Body.String())
			}
			collision := fmt.Sprintf(`{"action":"opened","number":43,"repository":{"full_name":"acme/infra"}}`)
			if got := deliver(collision, "delivery-1", sign(collision), "pull_request"); got.Code != 409 {
				t.Fatalf("collision status = %d", got.Code)
			}
			if planCalls.Load() != 1 || sourceReads.Load() != 2 {
				t.Fatalf("provider calls: plans=%d source=%d", planCalls.Load(), sourceReads.Load())
			}
		})
	}
}

func sign(body string) string {
	mac := hmac.New(sha256.New, []byte("webhook-secret"))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

const success = `{"Error":null,"Failure":"","PlansDeleted":false,"ProjectResults":[{"Error":null,"Failure":"","Command":1,"PlanSuccess":{"TerraformOutput":"Plan: 1 to add."},"RepoRelDir":"infra","Workspace":"default","ProjectName":"sample"}]}`
const failure = `{"Error":null,"Failure":"","PlansDeleted":false,"ProjectResults":[{"Error":{},"Failure":"invalid configuration","Command":1,"RepoRelDir":"infra","Workspace":"default","ProjectName":"sample"}]}`
const missing = `{"Error":null,"Failure":"","PlansDeleted":false,"ProjectResults":[]}`
const discarded = `{"Error":null,"Failure":"","PlansDeleted":true,"ProjectResults":[{"Error":null,"Failure":"","Command":1,"PlanSuccess":{"TerraformOutput":"Plan: 1 to add."},"RepoRelDir":"infra","Workspace":"default","ProjectName":"sample"}]}`
