package projectserver

import (
	"fmt"
	"path/filepath"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/core"
	tailwindscan "github.com/inoxlang/inox/internal/css/tailwind/scan"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
)

// A cssGenerator generates CSS stylesheets (most of the time in the /static/gen directory).
// It is not shared between sessions.
type cssGenerator struct {
	fsEventSource  *fs_ns.FilesystemEventSource
	inoxChunkCache *parse.ChunkCache
	fls            *Filesystem
	session        *jsonrpc.Session
	staticDir      string
}

func newCssGenerator(session *jsonrpc.Session, fls *Filesystem) *cssGenerator {
	ctx := session.Context()

	evs, err := fs_ns.NewEventSourceWithFilesystem(ctx, fls, core.PathPattern("/..."))
	if err != nil {
		panic(err)
	}

	generator := &cssGenerator{
		inoxChunkCache: parse.NewChunkCache(),
		fls:            fls,
		session:        session,
		staticDir:      "/static",
	}

	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go generator.genTailwindcss()
		},
	})

	return generator
}

func (g *cssGenerator) InitialGenAndSetup() {
	g.genTailwindcss()
}

func (g *cssGenerator) genAll() {
	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			logs.Println(g.session.Client(), err)
		}
	}()

	//TODO: make more flexible

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	g.genTailwindcss()
}

func (g *cssGenerator) genTailwindcss() {
	defer utils.Recover()
	ctx := g.session.Context()

	rulesets, err := tailwindscan.ScanForTailwindRulesToInclude(ctx, g.fls, tailwindscan.Configuration{
		TopDirectories: []string{"/"},
		InoxChunkCache: g.inoxChunkCache,
	})

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	//Create or truncate tailwind.css.
	path := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.TAILWIND_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	linefeeds := []byte{'\n', '\n'}

	f.Write([]byte(layout.TAILWIND_CSS_STYLESHEET_EXPLANATION))

	for _, ruleset := range rulesets {
		f.Write(linefeeds)
		f.Write([]byte(ruleset.Node.String()))
	}
}
