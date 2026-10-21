// Package rhhub embeds the role-household hub (role-household/rh, the self-hosted platform) so
// the setup wizard can start one on this machine (core.StartLocalHub). The source is copied in
// at build time by `make hubsrc` (the release workflow runs it); a plain `go build` has only
// src/README.md, and the wizard then says it cannot start a hub and offers connecting to one.
package rhhub

import (
	"embed"
	"io/fs"

	"github.com/chenhg5/cc-connect/core"
)

//go:embed all:src
var srcFS embed.FS

func init() {
	sub, err := fs.Sub(srcFS, "src")
	if err != nil {
		return
	}
	core.RegisterHubSource(sub)
}
