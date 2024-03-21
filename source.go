package inox

import "embed"

//go:embed app go.mod go.sum internal
var CodebaseFS embed.FS
