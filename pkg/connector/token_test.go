package connector

import (
	"testing"
	"time"
)

func TestMetadataFromToken(t *testing.T) {
	token := &tokenResponse{
		AccessToken:           "ghu_test",
		RefreshToken:          "ghr_test",
		ExpiresIn:             3600,
		RefreshTokenExpiresIn: 86400,
	}
	viewer := &viewerInfo{NodeID: "U_kg", Login: "octocat", DatabaseID: 1}
	meta := metadataFromToken(token, viewer)
	if meta.AccessToken != "ghu_test" {
		t.Fatalf("unexpected access token: %s", meta.AccessToken)
	}
	if meta.NodeID != "U_kg" {
		t.Fatalf("unexpected node id: %s", meta.NodeID)
	}
	if time.Unix(meta.AccessExpiresAt, 0).Before(time.Now()) {
		t.Fatal("access token should expire in the future")
	}
}
