package hsanalysis

import (
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/parse"
)

type Parameters struct {
	HyperscriptProgram hscode.JSONMap
	LocationKind       hyperscriptCodeLocationKind
	Component          *Component //may be nil
	Chunk              *parse.ParsedChunkSource
	InoxNodePosition   parse.SourcePositionRange
}

type hyperscriptCodeLocationKind int

const (
	InoxJsAttribute hyperscriptCodeLocationKind = iota
	ComponentUnderscoreAttribute
	UnderscoreAttribute
	HyperscriptScriptElement
	HyperscriptScriptFile
)
