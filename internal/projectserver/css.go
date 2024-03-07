package projectserver

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/css"
	cssbundle "github.com/inoxlang/inox/internal/css/bundle"
	tailwindscan "github.com/inoxlang/inox/internal/css/tailwind/scan"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CSS_GENERATION_TEMP_BUF_INITIAL_CAPACITY = 100_000
)

// A cssGenerator generates CSS stylesheets (most of the time in the /static/gen directory).
// It is not shared between sessions.
type cssGenerator struct {
	fsEventSource  *fs_ns.FilesystemEventSource
	inoxChunkCache *parse.ChunkCache
	fls            *Filesystem
	session        *jsonrpc.Session
	staticDir      string

	lock sync.Mutex
	buf  []byte
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
		buf:            make([]byte, 0, CSS_GENERATION_TEMP_BUF_INITIAL_CAPACITY),
	}

	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go generator.genAll()
		},
	})

	return generator
}

func (g *cssGenerator) InitialGenAndSetup() {
	g.buf = g.buf[0:0:cap(g.buf)]
	g.genAll()
}

func (g *cssGenerator) genAll() {
	g.lock.Lock()
	defer g.lock.Unlock()

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
	g.genMainBundle()
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
		err := ruleset.Node.WriteTo(f)
		if err != nil {
			logs.Println(g.session.Client(), err)
			return
		}
	}
}

func (g *cssGenerator) genMainBundle() {
	ctx := g.session.Context()

	mainCSSPath := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.MAIN_CSS_FILENAME)

	stylesheet, err := cssbundle.Bundle(ctx, cssbundle.BundlingParams{
		InputFile:  mainCSSPath,
		Filesystem: g.fls,
	})

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	//Create or truncate main-bundle.min.css.
	path := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.MAIN_BUNDLE_MIN_CSS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	//Stringify the stylesheet.

	buff := bytes.NewBuffer(g.buf)
	err = stylesheet.WriteTo(buff)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	//Write the output to the filesystem.

	err = css.MinifyStream(bytes.NewReader(buff.Bytes()), f)
	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

}
