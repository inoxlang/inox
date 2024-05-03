package hsanalysis

import (
	"github.com/inoxlang/inox/internal/codebase/analysis/text"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
)

type analyzer struct {
	parameters Parameters

	//state

	inTellCommand bool

	//result
	errors   []Error
	warnings []Warning
}

func Analyze(params Parameters) ([]Error, []Warning, error) {
	checker := &analyzer{
		parameters: params,
	}

	criticalErr := hscode.Walk(params.Node, checker.preVisitHyperscriptNode, checker.postVisitHyperscriptNode)
	if criticalErr != nil {
		return nil, nil, criticalErr
	}

	return checker.errors, checker.warnings, nil
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
