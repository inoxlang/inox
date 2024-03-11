package codecompletion

import (
	"github.com/inoxlang/inox/internal/codebase/analysis"
	httpspec "github.com/inoxlang/inox/internal/globals/http_ns/spec"
)

type InputData struct {
	StaticFileURLPaths []string         //examples: /index.js, /index.css
	ServerAPI          *httpspec.API    //optional
	CodebaseAnalysis   *analysis.Result //optional
}
