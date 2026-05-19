package connector

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/hlog"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

func verifyGitHubSignature(secret string, body []byte, signature string) bool {
	if secret == "" || signature == "" {
		return false
	}
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sigHex := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

func (gc *GHDConnector) handleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if !verifyGitHubSignature(gc.Config.WebhookSecret, body, sig) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}
	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "ping":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
		return
	case "installation":
		gc.handleInstallationEvent(ctx, body)
	case "installation_repositories":
		gc.handleInstallationRepositoriesEvent(ctx, body)
	case "discussion":
		gc.handleDiscussionEvent(ctx, body)
	case "discussion_comment":
		gc.handleDiscussionCommentEvent(ctx, body)
	default:
		hlog.FromRequest(r).Debug().Str("event", eventType).Msg("Ignoring unhandled webhook event")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type installationEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	} `json:"installation"`
	Repositories []struct {
		ID int64 `json:"id"`
	} `json:"repositories"`
	Sender ghUser `json:"sender"`
}

func (gc *GHDConnector) handleInstallationEvent(ctx context.Context, body []byte) {
	var evt installationEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		gc.br.Log.Error().Err(err).Msg("Failed to parse installation webhook")
		return
	}
	switch evt.Action {
	case "created", "added":
		gc.saveInstallation(ctx, evt)
	case "deleted":
		_ = gc.deleteInstallation(ctx, evt.Installation.ID)
	}
}

func (gc *GHDConnector) saveInstallation(ctx context.Context, evt installationEvent) {
	loginID, err := gc.getUserLoginIDByGitHubUserID(ctx, evt.Sender.ID)
	if err != nil {
		gc.br.Log.Warn().Err(err).Int64("sender_id", evt.Sender.ID).Msg("No bridge login for GitHub user who installed the app")
		return
	}
	var repos []int64
	for _, r := range evt.Repositories {
		repos = append(repos, r.ID)
	}
	_ = gc.upsertInstallation(ctx, installationRecord{
		InstallationID: evt.Installation.ID,
		UserLoginID:    loginID,
		AccountLogin:   evt.Installation.Account.Login,
		Repos:          repos,
		UpdatedAt:      time.Now(),
	})
}

type installationRepositoriesEvent struct {
	Action             string `json:"action"`
	Installation       struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	RepositoriesAdded []struct {
		ID int64 `json:"id"`
	} `json:"repositories_added"`
	RepositoriesRemoved []struct {
		ID int64 `json:"id"`
	} `json:"repositories_removed"`
	Sender ghUser `json:"sender"`
}

func (gc *GHDConnector) handleInstallationRepositoriesEvent(ctx context.Context, body []byte) {
	var evt installationRepositoriesEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		gc.br.Log.Error().Err(err).Msg("Failed to parse installation_repositories webhook")
		return
	}
	rows, err := gc.br.DB.Query(ctx, `SELECT installation_id, user_login_id, account_login, repos FROM github_installation WHERE installation_id = $1`, evt.Installation.ID)
	if err != nil {
		return
	}
	defer rows.Close()
	if !rows.Next() {
		// Try to create from sender if missing.
		gc.saveInstallation(ctx, installationEvent{
			Action: "created",
			Installation: struct {
				ID      int64 `json:"id"`
				Account struct {
					Login string `json:"login"`
				} `json:"account"`
			}{ID: evt.Installation.ID},
			Sender: evt.Sender,
		})
		return
	}
	var rec installationRecord
	var reposJSON []byte
	if err := rows.Scan(&rec.InstallationID, &rec.UserLoginID, &rec.AccountLogin, &reposJSON); err != nil {
		return
	}
	_ = json.Unmarshal(reposJSON, &rec.Repos)
	repoSet := make(map[int64]bool)
	for _, id := range rec.Repos {
		repoSet[id] = true
	}
	for _, r := range evt.RepositoriesAdded {
		repoSet[r.ID] = true
	}
	for _, r := range evt.RepositoriesRemoved {
		delete(repoSet, r.ID)
	}
	rec.Repos = rec.Repos[:0]
	for id := range repoSet {
		rec.Repos = append(rec.Repos, id)
	}
	rec.UpdatedAt = time.Now()
	_ = gc.upsertInstallation(ctx, rec)
}

type discussionEvent struct {
	Action     string       `json:"action"`
	Discussion ghDiscussion `json:"discussion"`
	Repository ghRepo       `json:"repository"`
}

func (gc *GHDConnector) handleDiscussionEvent(ctx context.Context, body []byte) {
	var evt discussionEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		gc.br.Log.Error().Err(err).Msg("Failed to parse discussion webhook")
		return
	}
	switch evt.Action {
	case "created":
		logins, _ := gc.getUserLoginsForRepo(ctx, evt.Repository.ID)
		for _, loginID := range logins {
			gc.handleDiscussionCreated(ctx, evt.Discussion, evt.Repository, networkid.UserLoginID(loginID))
		}
	case "edited":
		gc.handleDiscussionEdited(ctx, evt.Discussion, evt.Repository)
	case "deleted", "closed", "locked":
		gc.handleDiscussionStateChange(ctx, evt.Action, evt.Discussion, evt.Repository)
	}
}

type discussionCommentEvent struct {
	Action     string       `json:"action"`
	Comment    ghComment    `json:"comment"`
	Discussion ghDiscussion `json:"discussion"`
	Repository ghRepo       `json:"repository"`
}

func (gc *GHDConnector) handleDiscussionCommentEvent(ctx context.Context, body []byte) {
	var evt discussionCommentEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		gc.br.Log.Error().Err(err).Msg("Failed to parse discussion_comment webhook")
		return
	}
	gc.handleDiscussionComment(ctx, evt.Action, evt.Comment, evt.Discussion, evt.Repository)
}
