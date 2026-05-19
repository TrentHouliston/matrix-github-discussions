package connector

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyGitHubSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"action":"created"}`)
	sig := signBody(secret, body)
	if !verifyGitHubSignature(secret, body, sig) {
		t.Fatal("expected valid signature")
	}
	if verifyGitHubSignature(secret, body, "sha256=deadbeef") {
		t.Fatal("expected invalid signature")
	}
	if verifyGitHubSignature("", body, sig) {
		t.Fatal("expected rejection with empty secret")
	}
}
