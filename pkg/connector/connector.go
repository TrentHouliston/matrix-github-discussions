package connector

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/hlog"
	"go.mau.fi/util/configupgrade"
	"go.mau.fi/util/exhttp"
	"go.mau.fi/util/requestlog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
)

type GHDConnector struct {
	br     *bridgev2.Bridge
	Config Config
}

var _ bridgev2.NetworkConnector = (*GHDConnector)(nil)

func (gc *GHDConnector) Init(bridge *bridgev2.Bridge) {
	gc.br = bridge
	if gc.Config.ReactionPollIntervalSeconds == 0 {
		gc.Config = defaultConfig()
	}
}

func (gc *GHDConnector) Start(ctx context.Context) error {
	if err := gc.migrateInstallationTable(ctx); err != nil {
		return fmt.Errorf("failed to migrate installation table: %w", err)
	}
	server, ok := gc.br.Matrix.(bridgev2.MatrixConnectorWithServer)
	if !ok {
		return fmt.Errorf("matrix connector does not implement MatrixConnectorWithServer")
	} else if server.GetPublicAddress() == "" {
		return fmt.Errorf("public address of bridge not configured")
	}
	router := http.NewServeMux()
	router.HandleFunc("POST /webhook", gc.handleWebhook)
	server.GetRouter().Handle("/_ghdiscussions/", exhttp.ApplyMiddleware(
		router,
		exhttp.StripPrefix("/_ghdiscussions"),
		hlog.NewHandler(gc.br.Log.With().Str("component", "github webhooks").Logger()),
		requestlog.AccessLogger(requestlog.Options{TrustXForwardedFor: true}),
	))
	return nil
}

func (gc *GHDConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 1, 1
}

func (gc *GHDConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{}
}

func (gc *GHDConnector) GetName() bridgev2.BridgeName {
	return bridgev2.BridgeName{
		DisplayName:      "GitHub Discussions",
		NetworkURL:       "https://github.com",
		NetworkIcon:      "mxc://maunium.net/EPZUcFocxLVYJULAdyLjfAqC",
		NetworkID:        "github-discussions",
		BeeperBridgeType: "go.mau.fi/mautrix-ghdiscussions",
		DefaultPort:      29348,
		DefaultCommandPrefix: "!gh",
	}
}

func (gc *GHDConnector) GetConfig() (example string, data any, upgrader configupgrade.Upgrader) {
	return ExampleConfig, &gc.Config, configupgrade.SimpleUpgrader(upgradeConfig)
}

func (gc *GHDConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Portal: func() any {
			return &PortalMetadata{}
		},
		Message: func() any {
			return &MessageMetadata{}
		},
		UserLogin: func() any {
			return &UserLoginMetadata{}
		},
	}
}

func (gc *GHDConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	meta := login.Metadata.(*UserLoginMetadata)
	login.Client = &GHDClient{
		UserLogin: login,
		connector: gc,
		graphql:   newGraphQLClient(meta.AccessToken),
	}
	return nil
}

func (gc *GHDConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return []bridgev2.LoginFlow{{
		Name:        "Sign in with GitHub",
		Description: "Authorize via GitHub device flow (OAuth)",
		ID:          "device-flow",
	}}
}

func (gc *GHDConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID != "device-flow" {
		return nil, fmt.Errorf("unknown login flow ID %q", flowID)
	}
	return &GHDLogin{User: user, Connector: gc}, nil
}
