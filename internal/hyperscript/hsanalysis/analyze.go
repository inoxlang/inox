package hsanalysis

import (
	"slices"
	"strings"
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
	a := analyzerPool.Get().(*analyzer)
	a.parameters = params

	defer func() {
		*a = analyzer{}
		analyzerPool.Put(a)
	}()

	criticalErr := hscode.Walk(params.ProgramOrExpression, a.preVisitHyperscriptNode, a.postVisitHyperscriptNode)
	if criticalErr != nil {
		return nil, nil, criticalErr
	}

	return a.errors, a.warnings, nil
}

func (c *analyzer) preVisitHyperscriptNode(
	node hscode.JSONMap,
	nodeType hscode.NodeType,

	parent hscode.JSONMap,
	parentNodeType hscode.NodeType,

	ancestorChain []hscode.JSONMap,
	_ bool,
) (action hscode.AstTraversalAction, err error) {

	action = hscode.ContinueAstTraversal
	component := c.parameters.Component
	locationKind := c.parameters.LocationKind
	isInClientSideInterpolation := (locationKind == ClientSideAttributeInterpolation || locationKind == ClientSideTextInterpolation)
	inComponentContext := component != nil && isInClientSideInterpolation

	switch nodeType {
	case hscode.SetCommand:
	case hscode.TellCommand:
		if c.inTellCommand {
			return hscode.PruneAstTraversal, nil
		}
		c.inTellCommand = true
	case hscode.Symbol:
		if c.inTellCommand {
			c.addError(node, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD)
			return
		}
		name := hscode.GetSymbolName(node)
		if strings.HasPrefix(name, ":") {

			switch locationKind {
			case ClientSideAttributeInterpolation, ClientSideTextInterpolation:
				if component == nil {
					c.addError(node, text.ELEMENT_SCOPE_VARS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT)
				} else if !slices.Contains(component.InitialElementScopeVarNames, name) && !hscode.IsTarget(node, parent) {
					c.addError(node, text.FmtElementScopeVarMayNotBeDefined(name, inComponentContext))
				}
			default:
			}

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
