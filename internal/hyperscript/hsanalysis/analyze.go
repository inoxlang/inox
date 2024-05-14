package hsanalysis

import (
	"slices"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis/text"
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
	errors              []Error
	warnings            []Warning
	behaviors           []Behavior
	functionDefinitions []FunctionDefinition
}

type Result struct {
	Errors              []Error
	Warnings            []Warning
	Behaviors           []Behavior
	FunctionDefinitions []FunctionDefinition
}

func Analyze(params Parameters) (*Result, error) {
	a := analyzerPool.Get().(*analyzer)
	a.parameters = params

	defer func() {
		*a = analyzer{}
		analyzerPool.Put(a)
	}()

	criticalErr := hscode.Walk(params.ProgramOrExpression, a.preVisitHyperscriptNode, a.postVisitHyperscriptNode)
	if criticalErr != nil {
		return nil, criticalErr
	}

	return &Result{
		Warnings:            a.warnings,
		Errors:              a.errors,
		Behaviors:           a.behaviors,
		FunctionDefinitions: a.functionDefinitions,
	}, nil
}

func (a *analyzer) preVisitHyperscriptNode(
	node hscode.JSONMap,
	nodeType hscode.NodeType,

	parent hscode.JSONMap,
	parentNodeType hscode.NodeType,

	ancestorChain []hscode.JSONMap,
	_ bool,
) (action hscode.AstTraversalAction, err error) {

	action = hscode.ContinueAstTraversal
	component := a.parameters.Component
	locationKind := a.parameters.LocationKind
	isInClientSideInterpolation := (locationKind == ClientSideAttributeInterpolation || locationKind == ClientSideTextInterpolation)
	inComponentContext := component != nil && isInClientSideInterpolation

	switch nodeType {
	case hscode.SetCommand:
	case hscode.TellCommand:
		if a.inTellCommand {
			return hscode.PruneAstTraversal, nil
		}
		a.inTellCommand = true
	case hscode.Symbol:
		if a.inTellCommand {
			a.addError(node, text.VAR_NOT_IN_ELEM_SCOPE_OF_ELEM_REF_BY_TELL_CMD)
			return
		}
		name := hscode.GetSymbolName(node)
		if strings.HasPrefix(name, ":") {

			switch locationKind {
			case ClientSideAttributeInterpolation, ClientSideTextInterpolation:
				if component == nil {
					a.addError(node, text.ELEMENT_SCOPE_VARS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT)
					return
				}
				if !slices.Contains(component.InitialElementScopeVarNames, name) && !hscode.IsTarget(node, parent) {
					a.addError(node, text.FmtElementScopeVarMayNotBeDefined(name, inComponentContext))
				}
			default:
			}
		}
	case hscode.AttributeRef:
		attrName := hscode.GetAttributeRefName(node)
		if a.inTellCommand {
			a.addError(node, text.ATTR_NOT_REF_TO_ATTR_OF_ELEM_REF_BY_TELL_CMD)
			return
		}

		switch locationKind {
		case ClientSideAttributeInterpolation, ClientSideTextInterpolation:
			if component == nil {
				a.addError(node, text.ATTR_REFS_NOT_ALLOWED_HERE_BECAUSE_NO_COMPONENT)
				return
			}

			if !slices.Contains(component.InitializedDataAttributeNames, attrName) && !hscode.IsTarget(node, parent) {
				a.addError(node, text.FmtAttributeMayNotBeInitialized(attrName, inComponentContext))
			}
		default:
		}
	case hscode.DefFeature:
		a.functionDefinitions = append(a.functionDefinitions, MakeFunctionDefinitionFromDefFeature(node))
	case hscode.BehaviorFeature:
		a.behaviors = append(a.behaviors, MakeBehaviorFromBehaviorFeature(node))
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
