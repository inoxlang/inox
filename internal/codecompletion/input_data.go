package codecompletion

import (
	"github.com/inoxlang/inox/internal/codebase/analysis"
)

type InputData struct {
	StaticFileURLPaths []string         //examples: /index.js, /index.css
	CodebaseAnalysis   *analysis.Result //optional
}
