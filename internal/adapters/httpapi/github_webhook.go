package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/lanej/statecraft/internal/domain"
	"github.com/lanej/statecraft/internal/service"
)

type ProposedPlanner interface {
	Plan(context.Context, domain.RepositoryRef, int64, []domain.RootSelector) (service.ProposedPlan, error)
}

// GitHubWebhook is a synchronous acceptance-harness ingress. Delivery receipts
// are process-local; production requires durable intake and queued planning.
type GitHubWebhook struct {
	secret     []byte
	planner    ProposedPlanner
	roots      map[string][]domain.RootSelector
	mu         sync.Mutex
	deliveries map[string]deliveryReceipt
}

// PlanningReceipt retains both command evidence and an operation failure. It is
// available on identical redelivery without repeating the provider operation.
type PlanningReceipt struct {
	Delivery string                `json:"delivery"`
	State    string                `json:"state"`
	Result   *service.ProposedPlan `json:"result,omitempty"`
	Failure  string                `json:"failure,omitempty"`
}

type deliveryReceipt struct {
	digest [32]byte
	status int
	body   []byte
}

func writeReceipt(w http.ResponseWriter, receipt deliveryReceipt) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(receipt.status)
	w.Write(receipt.body)
}

func NewGitHubWebhook(secret string, planner ProposedPlanner, roots map[string][]domain.RootSelector) *GitHubWebhook {
	configured := make(map[string][]domain.RootSelector, len(roots))
	for repo, selectors := range roots {
		configured[repo] = append([]domain.RootSelector(nil), selectors...)
	}
	return &GitHubWebhook{secret: []byte(secret), planner: planner, roots: configured, deliveries: make(map[string]deliveryReceipt)}
}

func (h *GitHubWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if len(h.secret) == 0 {
		http.Error(w, "webhook is not configured", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid webhook body", http.StatusBadRequest)
		return
	}
	signature := r.Header.Get("X-Hub-Signature-256")
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	mac := hmac.New(sha256.New, h.secret)
	mac.Write(body)
	if err != nil || !strings.HasPrefix(signature, "sha256=") || !hmac.Equal(provided, mac.Sum(nil)) {
		http.Error(w, "invalid webhook signature", http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-GitHub-Event") != "pull_request" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var event struct {
		Action     string `json:"action"`
		Number     int64  `json:"number"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if json.Unmarshal(body, &event) != nil || event.Number <= 0 {
		http.Error(w, "invalid pull request event", http.StatusBadRequest)
		return
	}
	switch event.Action {
	case "opened", "synchronize", "reopened", "ready_for_review":
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}
	roots, ok := h.roots[event.Repository.FullName]
	parts := strings.Split(event.Repository.FullName, "/")
	if !ok || len(roots) == 0 || len(parts) != 2 {
		http.Error(w, "repository is not configured", http.StatusForbidden)
		return
	}
	delivery := r.Header.Get("X-GitHub-Delivery")
	if delivery == "" {
		http.Error(w, "delivery identity required", http.StatusBadRequest)
		return
	}
	digest := sha256.Sum256(body)
	h.mu.Lock()
	previous, duplicate := h.deliveries[delivery]
	if !duplicate && len(h.deliveries) < 4096 {
		pending, _ := json.Marshal(PlanningReceipt{Delivery: delivery, State: "pending"})
		h.deliveries[delivery] = deliveryReceipt{digest: digest, status: http.StatusAccepted, body: pending}
	} else if !duplicate {
		h.mu.Unlock()
		http.Error(w, "delivery capacity reached", http.StatusServiceUnavailable)
		return
	}
	h.mu.Unlock()
	if duplicate {
		if previous.digest != digest {
			http.Error(w, "delivery identity collision", http.StatusConflict)
			return
		}
		// Return the same evidence/failure, or pending while dispatch is in flight.
		writeReceipt(w, previous)
		return
	}
	result, err := h.planner.Plan(r.Context(), domain.RepositoryRef{Owner: parts[0], Name: parts[1]}, event.Number, roots)
	receipt := PlanningReceipt{Delivery: delivery, State: "completed", Result: &result}
	status := http.StatusOK
	if err != nil {
		// Provider error strings can include credentials/raw bodies. Keep a bounded
		// failure marker alongside the retained evidence rather than exposing them.
		receipt.State = "failed"
		receipt.Failure = "planning or freshness check failed; reconcile retained evidence before retry"
		status = http.StatusBadGateway
	}
	body, encodeErr := json.Marshal(receipt)
	if encodeErr != nil {
		body = []byte(`{"state":"failed","failure":"operation receipt encoding failed"}`)
		status = http.StatusInternalServerError
	}
	completed := deliveryReceipt{digest: digest, status: status, body: body}
	h.mu.Lock()
	h.deliveries[delivery] = completed
	h.mu.Unlock()
	writeReceipt(w, completed)
}
