package connector

import (
	"context"
	"fmt"
	"time"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

type GHDLogin struct {
	User      *bridgev2.User
	Connector *GHDConnector
	device    *deviceCodeResponse
}

var _ bridgev2.LoginProcessDisplayAndWait = (*GHDLogin)(nil)

func (gl *GHDLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	if gl.Connector.Config.ClientID == "" {
		return nil, fmt.Errorf("network.client_id is not configured")
	}
	device, err := requestDeviceCode(ctx, gl.Connector.Config.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}
	gl.device = device
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeDisplayAndWait,
		StepID:       "go.mau.fi.ghdiscussions.device_flow",
		Instructions: fmt.Sprintf("Visit %s and enter code %s to authorize the bridge with GitHub.", device.VerificationURI, device.UserCode),
		DisplayAndWaitParams: &bridgev2.LoginDisplayAndWaitParams{
			Type: bridgev2.LoginDisplayTypeCode,
			Data: device.UserCode,
		},
	}, nil
}

func (gl *GHDLogin) Wait(ctx context.Context) (*bridgev2.LoginStep, error) {
	if gl.device == nil {
		return nil, fmt.Errorf("device flow not started")
	}
	interval := time.Duration(gl.device.Interval) * time.Second
	token, err := pollDeviceToken(ctx, gl.Connector.Config.ClientID, gl.device.DeviceCode, interval)
	if err != nil {
		return nil, fmt.Errorf("device flow failed: %w", err)
	}
	gql := newGraphQLClient(token.AccessToken)
	viewer, err := gql.getViewer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query viewer: %w", err)
	}
	meta := metadataFromToken(token, viewer)
	ul, err := gl.User.NewLogin(ctx, &database.UserLogin{
		ID:         networkid.UserLoginID(viewer.NodeID),
		RemoteName: viewer.Login,
		Metadata:   meta,
	}, &bridgev2.NewLoginParams{
		LoadUserLogin: gl.Connector.LoadUserLogin,
	})
	if err != nil {
		return nil, err
	}
	installURL := "https://github.com/settings/installations"
	if gl.Connector.Config.AppSlug != "" {
		installURL = fmt.Sprintf("https://github.com/apps/%s/installations/new", gl.Connector.Config.AppSlug)
	}
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeComplete,
		StepID:       "go.mau.fi.ghdiscussions.complete",
		Instructions: fmt.Sprintf("Successfully logged in as @%s. Install the GitHub App on repositories you want to bridge: %s", viewer.Login, installURL),
		CompleteParams: &bridgev2.LoginCompleteParams{
			UserLoginID: ul.ID,
			UserLogin:   ul,
		},
	}, nil
}

func (gl *GHDLogin) Cancel() {}
