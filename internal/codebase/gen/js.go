package gen

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	hxgen "github.com/inoxlang/inox/internal/htmx/gen"
	"github.com/inoxlang/inox/internal/hyperscript/hsgen"
	ixgen "github.com/inoxlang/inox/internal/inoxjs/gen"
	"github.com/inoxlang/inox/internal/js"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const (
	JS_BUNDLING_BUFFER_SIZE = 100_000
)

// A JsGenerator generates JS files (most of the time in the /static/gen directory).
// It is not shared between sessions.
type JsGenerator struct {
	owner          string
	inoxChunkCache *parse.ChunkCache
	fls            afs.Filesystem
	staticDir      string
}

func NewJSGenerator(fls afs.Filesystem, staticDir, owner string) *JsGenerator {

	generator := &JsGenerator{
		inoxChunkCache: parse.NewChunkCache(),
		fls:            fls,
		owner:          owner,
		staticDir:      staticDir,
	}

	return generator
}

func (g *JsGenerator) InitialGenAndSetup(ctx *core.Context, analysis *analysis.Result) {
	g.RegenAll(ctx, analysis)
}

func (g *JsGenerator) RegenAll(ctx *core.Context, analysis *analysis.Result) {
	defer utils.Recover()

	//TODO: make more flexible

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	concatenated := &bytes.Buffer{}

	g.genHyperscript(ctx, analysis, concatenated)
	concatenated.WriteByte(';')

	g.genHTMX(ctx, analysis, concatenated)
	concatenated.WriteByte(';')

	g.genInox(ctx, analysis, concatenated)
	concatenated.WriteByte(';')

	g.writeBundle(concatenated)
}

func (g *JsGenerator) genHyperscript(ctx *core.Context, analysis *analysis.Result, bundleWriter io.Writer) {
	defer utils.Recover()

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.HYPERSCRIPTJS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	f.Write([]byte(layout.HYPERSCRIPT_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	defs := maps.Values(analysis.UsedHyperscriptFeatures)
	defs = append(defs, maps.Values(analysis.UsedHyperscriptCommands)...)

	if len(defs) > 0 {
		jsCode, err := hsgen.Generate(hsgen.Config{
			RequiredDefinitions: defs,
		})

		if err != nil {
			logs.Println(g.owner, err)
			return
		}

		w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
		w.Write(utils.StringAsBytes(jsCode))
	} else {
		f.Write(utils.StringAsBytes("\n/* This file is empty because no Hyperscript features or commands are used. */"))
	}
}

func (g *JsGenerator) genHTMX(ctx *core.Context, analysis *analysis.Result, bundleWriter io.Writer) {
	defer utils.Recover()

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.HTMX_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	extensions := maps.Keys(analysis.UsedHtmxExtensions)
	slices.SortFunc(extensions, func(a, b string) int {
		return strings.Compare(a, b)
	})

	jsCode, err := hxgen.Generate(hxgen.Config{
		Extensions: extensions,
	})
	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	f.Write([]byte(layout.HTMX_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
	w.Write(utils.StringAsBytes(jsCode))
}

func (g *JsGenerator) genInox(ctx *core.Context, analysis *analysis.Result, bundleWriter io.Writer) {
	defer utils.Recover()

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.INOX_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	jsCode, err := ixgen.Generate(ixgen.Config{
		Libraries: analysis.UsedInoxJsLibs,
	})
	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	f.Write([]byte(layout.INOX_JS_EXPLANATION))
	f.Write([]byte{'\n'})

	w := io.MultiWriter(f, bundleWriter) //write to the file and the bundle.
	w.Write(utils.StringAsBytes(jsCode))
}

func (g *JsGenerator) writeBundle(concatenatedJsFiles *bytes.Buffer) {
	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	path := filepath.Join(g.staticDir, layout.STATIC_JS_DIRNAME, layout.GLOBAL_BUNDLE_MIN_JS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	err = js.MinifyStream(concatenatedJsFiles, f, nil)
	if err != nil {
		logs.Println("bundle minification and writing", g.owner, err)
	}
}
