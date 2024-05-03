package hsanalysis

import (
	"sync"

	"github.com/inoxlang/inox/internal/codebase/analysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

var analyzerPool = sync.Pool{
	New: func() any {
		return &analyzer{}
	},
}

type analyzer struct {
	parameters Parameters

	//state

	inTellCommand bool

	//result
	errors   []Error
	warnings []Warning
}

func Analyze(params Parameters) ([]Error, []Warning, error) {
	analyzer := analyzerPool.Get().(*analyzer)
	analyzer.parameters = params

	defer func() {
		analyzer.parameters = Parameters{}
		analyzer.errors = nil
		analyzer.warnings = nil

		analyzerPool.Put(analyzer)
	}()

	criticalErr := hscode.Walk(params.HyperscriptProgram, analyzer.preVisitHyperscriptNode, analyzer.postVisitHyperscriptNode)
	if criticalErr != nil {
		return nil, nil, criticalErr
	}

	return analyzer.errors, analyzer.warnings, nil
}

func (c *analyzer) preVisitHyperscriptNode(
	node hscode.JSONMap,
	nodeType hscode.NodeType,

	parent hscode.JSONMap,
	parentNodeType hscode.NodeType,

	ancestorChain []hscode.JSONMap,
	_ bool,
) (hscode.AstTraversalAction, error) {

	switch nodeType {
	case hscode.SetCommand:
	case hscode.TellCommand:
		if c.inTellCommand {
			return hscode.PruneAstTraversal, nil
		}
		c.inTellCommand = true
	case hscode.Symbol:
		if hscode.IsTarget(node, parent) && c.inTellCommand {
			c.addError(node, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD)
		}
	}

	return hscode.ContinueAstTraversal, nil
}

func (c *analyzer) postVisitHyperscriptNode(
	node hscode.JSONMap,
	nodeType hscode.NodeType,

	parent hscode.JSONMap,
	parentNodeType hscode.NodeType,

	ancestorChain []hscode.JSONMap,
	_ bool,
) (hscode.AstTraversalAction, error) {

	switch nodeType {
	case hscode.TellCommand:
		c.inTellCommand = false
	}

	return hscode.ContinueAstTraversal, nil
}
