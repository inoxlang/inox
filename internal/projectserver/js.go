package projectserver

import (
	"bytes"
	"io"
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	hxgen "github.com/inoxlang/inox/internal/htmx/gen"
	hxscan "github.com/inoxlang/inox/internal/htmx/scan"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	hsscan "github.com/inoxlang/inox/internal/hyperscript/scan"
	"github.com/inoxlang/inox/internal/inoxconsts"
	ixgen "github.com/inoxlang/inox/internal/inoxjs/gen"
	ixscan "github.com/inoxlang/inox/internal/inoxjs/scan"
	"github.com/inoxlang/inox/internal/js"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	JS_BUNDLING_BUFFER_SIZE = 100_000
)

// A jsGenerator generates JS files (most of the time in the /static/gen directory).
// It is not shared between sessions.
type jsGenerator struct {
	fsEventSource  *fs_ns.FilesystemEventSource
	inoxChunkCache *parse.ChunkCache
	fls            *Filesystem
	session        *jsonrpc.Session
	staticDir      string
}

func newJSGenerator(session *jsonrpc.Session, fls *Filesystem) *jsGenerator {
	ctx := session.Context()

	evs, err := fs_ns.NewEventSourceWithFilesystem(ctx, fls, core.PathPattern("/..."))
	if err != nil {
		panic(err)
	}

	generator := &jsGenerator{
		inoxChunkCache: parse.NewChunkCache(),
		fls:            fls,
		session:        session,
		staticDir:      "/static/",
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

func (g *jsGenerator) InitialGenAndSetup() {
	g.genAll()
}

func (g *jsGenerator) genAll() {
	defer utils.Recover()

	//TODO: make more flexible

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	concatenated := &bytes.Buffer{}

	g.genHyperscript(concatenated)
	concatenated.WriteByte(';')

	g.genHTMX(concatenated)
	concatenated.WriteByte(';')

	g.genInox(concatenated)
	concatenated.WriteByte(';')

	g.writeBundle(concatenated)
}

func (g *jsGenerator) genHyperscript(bundleWriter io.Writer) {
	defer utils.Recover()

	//Find used features and commands.

	scanResult, err := hsscan.ScanCodebase(g.session.Context(), g.fls, hsscan.Configuration{
		TopDirectories: []string{"/"},
		InoxChunkCache: g.inoxChunkCache,
	})

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	//TODO: make more flexible

	err = g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.HYPERSCRIPTJS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	jsCode, err := hsgen.Generate(hsgen.Config{
		RequiredDefinitions: scanResult.RequiredDefinitions,
	})
	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	f.Write([]byte(layout.HYPERSCRIPT_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
	w.Write(utils.StringAsBytes(jsCode))
}

func (g *jsGenerator) genHTMX(bundleWriter io.Writer) {
	defer utils.Recover()

	//Find used features and commands.

	scanResult, err := hxscan.ScanCodebase(g.session.Context(), g.fls, hxscan.Configuration{
		TopDirectories: []string{"/"},
		InoxChunkCache: g.inoxChunkCache,
	})

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.HTMX_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	jsCode, err := hxgen.Generate(hxgen.Config{
		Extensions: scanResult.UsedExtensions,
	})
	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	f.Write([]byte(layout.HTMX_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
	w.Write(utils.StringAsBytes(jsCode))
}

func (g *jsGenerator) genInox(bundleWriter io.Writer) {
	defer utils.Recover()

	//Find used features and commands.

	scanResult, err := ixscan.ScanCodebase(g.session.Context(), g.fls, ixscan.Configuration{
		TopDirectories: []string{"/"},
		InoxChunkCache: g.inoxChunkCache,
	})

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	//TODO: make more flexible

	err = g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.INOX_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	jsCode, err := ixgen.Generate(ixgen.Config{
		Libraries: scanResult.Libraries,
	})
	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	f.Write([]byte(layout.INOX_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
	w.Write(utils.StringAsBytes(jsCode))
}

func (g *jsGenerator) writeBundle(concatenatedJsFiles *bytes.Buffer) {
	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.GLOBAL_BUNDLE_MIN_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	defer f.Close()

	err = js.MinifyStream(concatenatedJsFiles, f, nil)
	if err != nil {
		logs.Println("bundle minification and writing", g.session.Client(), err)
	}
}
