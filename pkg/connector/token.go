package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"maunium.net/go/mautrix/bridgev2/status"
)

const (
	githubDeviceCodeURL = "https://github.com/login/device/code"
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	tokenRefreshSkew    = 60 * time.Second
)

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenResponse struct {
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	Scope                 string `json:"scope"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Error                 string `json:"error"`
	ErrorDescription      string `json:"error_description"`
}

func requestDeviceCode(ctx context.Context, clientID string) (*deviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubDeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("device code request failed: %s", string(body))
	}
	var out deviceCodeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func pollDeviceToken(ctx context.Context, clientID, deviceCode string, interval time.Duration) (*tokenResponse, error) {
	if interval < time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			token, err := exchangeToken(ctx, url.Values{
				"client_id":   {clientID},
				"device_code": {deviceCode},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			})
			if err != nil {
				if strings.Contains(err.Error(), "authorization_pending") {
					continue
				}
				if strings.Contains(err.Error(), "slow_down") {
					interval += 5 * time.Second
					ticker.Reset(interval)
					continue
				}
				return nil, err
			}
			return token, nil
		}
	}
}

func refreshUserToken(ctx context.Context, clientID, refreshToken string) (*tokenResponse, error) {
	return exchangeToken(ctx, url.Values{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func exchangeToken(ctx context.Context, data url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out tokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		if out.Error == "authorization_pending" || out.Error == "slow_down" {
			return nil, fmt.Errorf("%s", out.Error)
		}
		desc := out.ErrorDescription
		if desc == "" {
			desc = out.Error
		}
		return nil, fmt.Errorf("token error: %s", desc)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}
	return &out, nil
}

func metadataFromToken(token *tokenResponse, viewer *viewerInfo) *UserLoginMetadata {
	now := time.Now()
	return &UserLoginMetadata{
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		AccessExpiresAt:  now.Add(time.Duration(token.ExpiresIn) * time.Second).Unix(),
		RefreshExpiresAt: now.Add(time.Duration(token.RefreshTokenExpiresIn) * time.Second).Unix(),
		Login:            viewer.Login,
		DatabaseID:       viewer.DatabaseID,
		NodeID:           viewer.NodeID,
	}
}

func (c *GHDClient) ensureValidToken(ctx context.Context) error {
	meta := c.UserLogin.Metadata.(*UserLoginMetadata)
	expiresAt := time.Unix(meta.AccessExpiresAt, 0)
	if time.Now().Add(tokenRefreshSkew).Before(expiresAt) {
		return nil
	}
	if meta.RefreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token available")
	}
	token, err := refreshUserToken(ctx, c.connector.Config.ClientID, meta.RefreshToken)
	if err != nil {
		c.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateBadCredentials,
			Error:      "github-token-refresh-failed",
			Message:    "Failed to refresh GitHub token",
			Info:       map[string]any{"go_error": err.Error()},
		})
		return err
	}
	*meta = *metadataFromToken(token, &viewerInfo{
		Login:      meta.Login,
		DatabaseID: meta.DatabaseID,
		NodeID:     meta.NodeID,
	})
	c.graphql = newGraphQLClient(token.AccessToken)
	return c.UserLogin.Save(ctx)
}
