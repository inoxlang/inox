package projectserver

import (
	"path/filepath"

	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	tailwindscan "github.com/inoxlang/inox/internal/tailwind/scan"
)

// A cssGenerator generates CSS stylesheets (most of the time in the /static/gen directory).
// It is not shared between sessions.
type cssGenerator struct {
}

func newCssGenerator() *cssGenerator {
	return &cssGenerator{}
}

func (g *cssGenerator) InitialGen(session *jsonrpc.Session, fls *Filesystem) {
	g.gen(session, fls)
}

func (g *cssGenerator) gen(session *jsonrpc.Session, fls *Filesystem) {
	ctx := session.Context()

	rulesets, err := tailwindscan.ScanForTailwindRulesToInclude(ctx, fls, tailwindscan.Configuration{
		TopDirectories: []string{"/"},
	})

	if err != nil {
		logs.Println(session.Client(), err)
		return
	}

	//TODO: make more flexible
	path := filepath.Join("/static/", scaffolding.RELATIVE_TAILWIND_FILE_PATH)

	f, err := fls.Create(path)

	if err != nil {
		logs.Println(session.Client(), err)
		return
	}

	defer f.Close()

	linefeeds := []byte{'\n', '\n'}

	f.Write([]byte(scaffolding.EMPTY_TAILWIND_CSS_STYLESHEET))

	for _, ruleset := range rulesets {
		f.Write(linefeeds)
		f.Write([]byte(ruleset.Node.String()))
	}
}
