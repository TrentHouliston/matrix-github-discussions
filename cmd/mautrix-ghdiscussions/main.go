package main

import (
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"

	"go.mau.fi/mautrix-ghdiscussions/pkg/connector"
)

// Information to find out exactly which commit the bridge was built from.
// These are filled at build time with the -X linker flag.
var (
	Tag       = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	m := mxmain.BridgeMain{
		Name:        "mautrix-ghdiscussions",
		Description: "A Matrix-GitHub Discussions bridge",
		URL:         "https://github.com/mautrix/ghdiscussions",
		Version:     "0.1.0",
		Connector:   &connector.GHDConnector{},
	}
	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}
