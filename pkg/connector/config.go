package connector

import (
	_ "embed"

	"go.mau.fi/util/configupgrade"
)

//go:embed example-config.yaml
var ExampleConfig string

// Config holds network-specific bridge configuration.
type Config struct {
	ClientID                    string `yaml:"client_id"`
	AppID                       int64  `yaml:"app_id"`
	PrivateKeyPath              string `yaml:"private_key_path"`
	WebhookSecret               string `yaml:"webhook_secret"`
	AppSlug                     string `yaml:"app_slug"`
	ReactionPollIntervalSeconds int    `yaml:"reaction_poll_interval_seconds"`
	ReactionPollActiveWindows   int    `yaml:"reaction_poll_active_windows"`
}

func upgradeConfig(helper configupgrade.Helper) {
	helper.Copy(configupgrade.Str, "client_id")
	helper.Copy(configupgrade.Int, "app_id")
	helper.Copy(configupgrade.Str, "private_key_path")
	helper.Copy(configupgrade.Str, "webhook_secret")
	helper.Copy(configupgrade.Str, "app_slug")
	helper.Copy(configupgrade.Int, "reaction_poll_interval_seconds")
	helper.Copy(configupgrade.Int, "reaction_poll_active_windows")
}

func defaultConfig() Config {
	return Config{
		ReactionPollIntervalSeconds: 60,
		ReactionPollActiveWindows:   5,
	}
}
