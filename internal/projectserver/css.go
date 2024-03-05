package projectserver

import (
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	tailwindscan "github.com/inoxlang/inox/internal/tailwind/scan"
	"github.com/inoxlang/inox/internal/utils"
)

// A cssGenerator generates CSS stylesheets (most of the time in the /static/gen directory).
// It is not shared between sessions.
type cssGenerator struct {
	fsEventSource *fs_ns.FilesystemEventSource
	fls           *Filesystem
}

func newCssGenerator() *cssGenerator {
	return &cssGenerator{}
}

func (g *cssGenerator) InitialGenAndSetup(session *jsonrpc.Session, fls *Filesystem) {
	ctx := session.Context()
	g.fls = fls

	g.gen(session)

	evs, err := fs_ns.NewEventSourceWithFilesystem(ctx, fls, core.PathPattern("/..."))
	if err != nil {
		panic(err)
	}
	g.fsEventSource = evs

	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go g.gen(session)
		},
	})
}

func (g *cssGenerator) gen(session *jsonrpc.Session) {
	defer utils.Recover()
	ctx := session.Context()

	rulesets, err := tailwindscan.ScanForTailwindRulesToInclude(ctx, g.fls, tailwindscan.Configuration{
		TopDirectories: []string{"/"},
	})

	if err != nil {
		logs.Println(session.Client(), err)
		return
	}

	//TODO: make more flexible
	path := filepath.Join("/static/", scaffolding.RELATIVE_TAILWIND_FILE_PATH)

	f, err := g.fls.Create(path)

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
