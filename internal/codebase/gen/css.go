package gen

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/css"
	cssbundle "github.com/inoxlang/inox/internal/css/bundle"
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/css/varclasses"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

const (
	CSS_GENERATION_TEMP_BUF_INITIAL_CAPACITY = 100_000
)

// A CssGenerator generates CSS stylesheets (most of the time in the /static/gen directory).
type CssGenerator struct {
	owner          string
	inoxChunkCache *parse.ChunkCache
	fls            afs.Filesystem
	staticDir      string

	lock sync.Mutex
	buf  []byte
}

func NewCssGenerator(fls afs.Filesystem, staticDir, owner string) *CssGenerator {
	generator := &CssGenerator{
		inoxChunkCache: parse.NewChunkCache(),
		fls:            fls,
		staticDir:      staticDir,
		owner:          owner,
		buf:            make([]byte, 0, CSS_GENERATION_TEMP_BUF_INITIAL_CAPACITY),
	}

	return generator
}

func (g *CssGenerator) InitialGenAndSetup(ctx *core.Context, analysis *analysis.Result) {
	g.RegenAll(ctx, analysis)
}

func (g *CssGenerator) RegenAll(ctx *core.Context, analysis *analysis.Result) {
	g.lock.Lock()
	defer g.lock.Unlock()

	g.buf = g.buf[0:0:cap(g.buf)]

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())
			logs.Println(g.owner, err)
		}
	}()

	//TODO: make more flexible

	err := g.fls.MkdirAll(filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME), 0700)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	g.genUtilities(ctx, analysis.UsedTailwindRules, analysis.UsedVarBasedCssRules)
	g.genMainBundle(ctx)
}

func (g *CssGenerator) genUtilities(
	ctx *core.Context,
	rulesets map[string]tailwind.Ruleset,
	varBasedCssClasses map[css.VarName]varclasses.Variable,
) {

	//Create or truncate utilities.css.
	path := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.UTILITY_CLASSES_FILENAME)
	linefeeds := []byte{'\n', '\n'}

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	f.Write([]byte(layout.UTILITY_CLASSES_STYLESHEET_EXPLANATION))

	//Var-based rulesets.

	vars := maps.Values(varBasedCssClasses)
	slices.SortFunc(vars, func(a, b varclasses.Variable) int {
		return strings.Compare(string(a.Name), string(b.Name))
	})

	for _, cssVar := range vars {
		f.Write(linefeeds)
		err := cssVar.AutoRuleset.WriteTo(f)
		if err != nil {
			logs.Println(g.owner, err)
			return
		}
	}

	//Tailwind rulesets.

	rulesetList := maps.Values(rulesets)

	err = tailwind.WriteRulesets(f, rulesetList)
	if err != nil {
		logs.Println(g.owner, err)
	}
}

func (g *CssGenerator) genMainBundle(ctx *core.Context) {

	mainCSSPath := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.MAIN_CSS_FILENAME)

	stylesheet, err := cssbundle.Bundle(ctx, cssbundle.BundlingParams{
		InputFile:  mainCSSPath,
		Filesystem: g.fls,
	})

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	//Create or truncate main-bundle.min.css.
	path := filepath.Join(g.staticDir, layout.STATIC_STYLES_DIRNAME, layout.MAIN_BUNDLE_MIN_CSS_FILENAME)

	f, err := g.fls.Create(path)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	defer f.Close()

	//Stringify the stylesheet.

	buff := bytes.NewBuffer(g.buf)
	err = stylesheet.WriteTo(buff)

	if err != nil {
		logs.Println(g.owner, err)
		return
	}

	//Write the output to the filesystem.

	err = css.MinifyStream(bytes.NewReader(buff.Bytes()), f)
	if err != nil {
		logs.Println(g.owner, err)
		return
	}

}
