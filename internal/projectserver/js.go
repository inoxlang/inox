package projectserver

import (
	"os"
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	hsscan "github.com/inoxlang/inox/internal/hyperscript/scan"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/js"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/scaffolding"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
)

// A jsGenerator generates JS files (most of the time in the /static/gen directory).
// It is not shared between sessions.
type jsGenerator struct {
	fsEventSource  *fs_ns.FilesystemEventSource
	inoxChunkCache *parse.ChunkCache
	fls            *Filesystem
	session        *jsonrpc.Session
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
	}

	evs.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: 2 * fs_ns.OLD_EVENT_MIN_AGE,
		IsIgnoredEvent: func(e *core.Event) (ignore bool) {
			fsEvent := e.SourceValue().(fs_ns.Event)

			ignore = !fsEvent.IsStructureOrContentChange() || fsEvent.Path().Extension() != inoxconsts.INOXLANG_FILE_EXTENSION
			return
		},
		Microtask: func() {
			go generator.gen()
		},
	})

	return generator
}

func (g *jsGenerator) InitialGenAndSetup() {
	g.gen()
}

func (g *jsGenerator) gen() {
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
	path := filepath.Join("/static/", scaffolding.RELATIVE_MINIFIED_HYPERSCRIPT_FILE_PATH)

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

	os.WriteFile("/tmp/out.xx", []byte(jsCode), 0600)

	minified, err := js.Minify(jsCode, nil)
	if err != nil {
		logs.Println(g.session.Client(), err)
		return
	}

	f.Write([]byte(scaffolding.HYPERSCRIPT_MIN_JS_EXPLANATION))
	f.Write([]byte{'\n', '\n'})
	f.Write(utils.StringAsBytes(minified))
}
