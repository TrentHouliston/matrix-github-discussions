package connector

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookPing(t *testing.T) {
	gc := &GHDConnector{Config: Config{WebhookSecret: "fixture-secret"}}
	body := []byte(`{"zen":"Design for failure."}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", signBody(gc.Config.WebhookSecret, body))
	w := httptest.NewRecorder()
	gc.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	gc := &GHDConnector{Config: Config{WebhookSecret: "fixture-secret"}}
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "discussion")
	req.Header.Set("X-Hub-Signature-256", "sha256=00")
	w := httptest.NewRecorder()
	gc.handleWebhook(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}
