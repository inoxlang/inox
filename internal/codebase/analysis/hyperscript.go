package analysis

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	html_symbolic "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxjs"
	"github.com/inoxlang/inox/internal/parse"
)

func (a *analyzer) addUsedHyperscriptFeaturesAndCommands(node parse.Node) {
	//Get tokens

	allUsedFeatures := a.result.UsedHyperscriptFeatures
	allUsedCommands := a.result.UsedHyperscriptCommands

	switch node := node.(type) {
	case *parse.HyperscriptAttributeShorthand:
		if node.HyperscriptParsingResult != nil {
			hsanalysis.AddUsedFeaturesAndCommands(node.HyperscriptParsingResult.NodeData, allUsedFeatures, allUsedCommands)

		} else if node.HyperscriptParsingError != nil {
			err := node.HyperscriptParsingError.Tokens
			hsanalysis.GuessUsedFeaturesAndCommandsFromTokens(err, allUsedFeatures, allUsedCommands)
		}
	case *parse.MarkupElement:
		if node.EstimatedRawElementType == parse.HyperscriptScript {
			result, ok := node.RawElementParsingResult.(*hscode.ParsingResult)
			if ok {
				hsanalysis.AddUsedFeaturesAndCommands(result.NodeData, allUsedFeatures, allUsedCommands)
			} else if err, ok := node.RawElementParsingResult.(*hscode.ParsingError); ok {
				hsanalysis.GuessUsedFeaturesAndCommandsFromTokens(err.Tokens, allUsedFeatures, allUsedCommands)
			}
		}
	}
}

func (a *analyzer) analyzeHyperscriptComponent(component *hsanalysis.Component) (criticalError error) {
	if a.ctx.IsDoneSlowCheck() {
		return a.ctx.Err()
	}

	//Retrieve symbolic data about the HTML node.

	var componentRootNode *html_symbolic.HTMLNode

	chunkName := component.ChunkSource.Name()
	module, ok := a.result.LocalModules[chunkName]
	if ok {
		val, _ := module.SymbolicData.GetMostSpecificNodeValue(component.ClosestMarkupExpr)
		markupExprValue, ok := val.(*html_symbolic.HTMLNode)
		if ok {
			componentRootNode, _ = markupExprValue.FindNode(component.Element.Span, component.ChunkSource.Name())
		}
	}

	//Analyze the Hyperscript code of the compontent's root node.

	errors, warnings, criticalError := hsanalysis.Analyze(hsanalysis.Parameters{
		HyperscriptProgram: component.AttributeShorthand.HyperscriptParsingResult.NodeData,
		LocationKind:       hsanalysis.UnderscoreAttribute,
		Component:          component,
		Chunk:              component.ChunkSource,
		InoxNodePosition:   component.ChunkSource.GetSourcePosition(component.AttributeShorthand.Span),
	})

	if criticalError != nil {
		return
	}

	a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, errors...)
	a.result.HyperscriptWarnings = append(a.result.HyperscriptWarnings, warnings...)

	if componentRootNode == nil {
		return
	}

	//Analyze the Hyperscript attribute shorthands of elements and client-side interpolations inside the component.

	visitedMarkupElements := map[*parse.MarkupElement]struct{}{}

	err := componentRootNode.Walk(func(node *html_symbolic.HTMLNode) (action html_symbolic.TraversalAction, criticalErr error) {
		action = html_symbolic.ContinueTraversal

		sourceNode, ok := node.SourceNode()
		if !ok {
			return
		}

		if _, ok := visitedMarkupElements[sourceNode.Node]; ok {
			return
		}

		err := parse.Walk(sourceNode.Node, func(node, parent, _ parse.Node, _ []parse.Node, _ bool) (action parse.TraversalAction, criticalErr error) {
			action = parse.ContinueTraversal

			markupElem, ok := node.(*parse.MarkupElement)
			if !ok { //we only care about AST nodes that may contain Hyperscript code.
				return
			}

			if _, ok := visitedMarkupElements[markupElem]; ok {
				action = parse.Prune
				return
			}

			visitedMarkupElements[markupElem] = struct{}{}

			if hsanalysis.IsHyperscriptComponent(markupElem) { //do no enter the sub-tree of descendant components.
				action = parse.Prune
				return
			}

			action = parse.Prune
			criticalErr = a.analyzeHyperscriptInMarkupElement(component, sourceNode)
			return
		}, nil)

		if err != nil {
			criticalErr = err
		}

		return
	}, nil)

	if err != nil {
		criticalError = err
	}

	return
}

func (a *analyzer) analyzeHyperscriptInMarkupElement(component *hsanalysis.Component, sourceNode *symbolic.MarkupSourceNode) (criticalErr error) {

	//Analyze attribute shorthand.

	attribute, ok := sourceNode.Node.HyperscriptAttributeShorthand()
	if ok && attribute.HyperscriptParsingResult != nil {
		errors, warnings, err := hsanalysis.Analyze(hsanalysis.Parameters{
			HyperscriptProgram: attribute.HyperscriptParsingResult.NodeData,
			LocationKind:       hsanalysis.UnderscoreAttribute,
			Component:          component,
			Chunk:              component.ChunkSource,
			InoxNodePosition:   component.ChunkSource.GetSourcePosition(attribute.Span),
		})

		if err != nil {
			criticalErr = err
			return
		}

		a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, errors...)
		a.result.HyperscriptWarnings = append(a.result.HyperscriptWarnings, warnings...)

		return
	}

	//Analyze client-side interpolatons in attributes.

	for _, attr := range sourceNode.Node.Opening.Attributes {
		attr, ok := attr.(*parse.MarkupAttribute)
		if !ok || attr.IsNameEqual(inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME) {
			continue
		}

		value := attr.ValueIfStringLiteral()
		inoxjs.ContainsClientSideInterpolation()
	}

	return
}
