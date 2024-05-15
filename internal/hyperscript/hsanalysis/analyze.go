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
	behaviorStack []*Behavior

	//result
	errors              []Error
	warnings            []Warning
	behaviors           []*Behavior
	functionDefinitions []FunctionDefinition
}

type Result struct {
	Errors              []Error
	Warnings            []Warning
	Behaviors           []*Behavior
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
		a.functionDefinitions = append(a.functionDefinitions, MakeFunctionDefinitionFromNode(node))
	case hscode.BehaviorFeature:
		behavior := MakeBehaviorFromNode(node)
		a.behaviors = append(a.behaviors, behavior)
		a.behaviorStack = append(a.behaviorStack, behavior)

		preAnalyzeFeaturesOfBehaviorOrComponent(
			&behavior.InitialElementScopeVarNames,
			&behavior.InitializedDataAttributeNames,
			&behavior.HandledEvents,
			&behavior.Installs,
			behavior.Features,
		)
	}

	return hscode.ContinueAstTraversal, nil
}

func (a *analyzer) postVisitHyperscriptNode(
	node hscode.JSONMap,
	nodeType hscode.NodeType,

	parent hscode.JSONMap,
	parentNodeType hscode.NodeType,

	ancestorChain []hscode.JSONMap,
	_ bool,
) (hscode.AstTraversalAction, error) {

	switch nodeType {
	case hscode.TellCommand:
		a.inTellCommand = false
	case hscode.BehaviorFeature:
		a.behaviorStack = a.behaviorStack[:len(a.behaviorStack)-1]
	}

	return hscode.ContinueAstTraversal, nil
}

func preAnalyzeFeaturesOfBehaviorOrComponent(
	initialElementScopeVarNames *[]string,
	initializedDataAttributeNames *[]string,
	handledEvents *[]DOMEvent,
	installs *[]*InstallFeature,
	features []any,
) {
	walk := func(node hscode.JSONMap, inInit bool) {
		hscode.Walk(node, func(node hscode.JSONMap, nodeType hscode.NodeType, _ hscode.JSONMap, _ hscode.NodeType, _ []hscode.JSONMap, _ bool) (hscode.AstTraversalAction, error) {
			switch nodeType {
			case hscode.SetCommand:
				target, _ := hscode.GetSetCommandTarget(node)
				switch hscode.GetTypeIfNode(target) {
				case hscode.Symbol:
					name := hscode.GetSymbolName(target)
					if inInit && strings.HasPrefix(name, ":") && !slices.Contains(*initialElementScopeVarNames, name) {
						*initialElementScopeVarNames = append(*initialElementScopeVarNames, name)
					}
				case hscode.AttributeRef:
					name := hscode.GetAttributeRefName(target)
					if inInit && strings.HasPrefix(name, "data-") && !slices.Contains(*initializedDataAttributeNames, name) {
						*initializedDataAttributeNames = append(*initializedDataAttributeNames, name)
					}
				}

			}
			return hscode.ContinueAstTraversal, nil
		}, nil)
	}

	for _, feature := range features {
		feature := feature.(hscode.JSONMap)
		switch hscode.GetTypeIfNode(feature) {
		case hscode.InitFeature: //init
			walk(feature, true)
		case hscode.OnFeature: //on
			onFeature := feature
			events, _ := hscode.GetOnFeatureEvents(onFeature)
			for _, event := range events {
				*handledEvents = append(*handledEvents, DOMEvent{
					Type: event.Name,
				})
			}
			walk(feature, false)
		case hscode.InstallFeature:
			installfeature := MakeInstallFeatureFromNode(feature)
			*installs = append(*installs, installfeature)
		}
	}
}
