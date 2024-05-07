package analysis

import (
	"strings"

	"github.com/inoxlang/inox/internal/codebase/analysis/text"
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

			if markupElem != component.Element && hsanalysis.IsHyperscriptComponent(markupElem) { //do no enter the sub-tree of descendant components.
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

		locationKind := hsanalysis.UnderscoreAttribute
		if sourceNode.Node == component.Element {
			locationKind = hsanalysis.ComponentUnderscoreAttribute
		}

		errors, warnings, err := hsanalysis.Analyze(hsanalysis.Parameters{
			HyperscriptProgram: attribute.HyperscriptParsingResult.NodeData,
			LocationKind:       locationKind,
			Component:          component,
			Chunk:              sourceNode.Chunk,
			InoxNodePosition:   sourceNode.Chunk.GetSourcePosition(attribute.Span),
		})

		if err != nil {
			criticalErr = err
			return
		}

		a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, errors...)
		a.result.HyperscriptWarnings = append(a.result.HyperscriptWarnings, warnings...)
	}

	analyzeInterpolationsInString := func(str string, encoded string, nodeSpan parse.NodeSpan) (criticalErr error) {
		if strings.Count(str, inoxjs.INTERPOLATION_OPENING_DELIMITER) != strings.Count(encoded, inoxjs.INTERPOLATION_OPENING_DELIMITER) ||
			strings.Count(str, inoxjs.INTERPOLATION_CLOSING_DELIMITER) != strings.Count(encoded, inoxjs.INTERPOLATION_CLOSING_DELIMITER) {

			analysisError := hsanalysis.MakeError(
				text.ATTRS_SHOULD_NOT_CONTAIN_ENCODED_CLIENT_SIDE_DELIMS,
				sourceNode.Chunk.GetSourcePosition(nodeSpan),
			)

			a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, analysisError)

			return
		}

		if strings.Count(str, inoxjs.INTERPOLATION_OPENING_DELIMITER) == 0 {
			//No interpolations.
			return
		}

		interpolations, err := inoxjs.ParseClientSideInterpolations(a.ctx, str, encoded)
		if err != nil {
			criticalErr = err
			return
		}

		for _, interp := range interpolations {
			if interp.ParsingError != nil {
				analysisError := hsanalysis.MakeError(
					interp.ParsingError.Message,
					//location
					sourceNode.Chunk.GetSourcePosition(parse.NodeSpan{
						Start: nodeSpan.Start + interp.StartRuneIndex,
						End:   nodeSpan.Start + interp.EndRuneIndex,
					}),
				)

				a.result.HyperscriptErrors = append(a.result.HyperscriptErrors, analysisError)
			}
		}
		return
	}

	//Analyze client-side interpolations in attributes.

	for _, attr := range sourceNode.Node.Opening.Attributes {
		attr, ok := attr.(*parse.MarkupAttribute)
		if !ok || attr.IsNameEqual(inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME) {
			continue
		}

		encoded := ""
		str := ""

		switch v := attr.Value.(type) {
		case *parse.DoubleQuotedStringLiteral:
			encoded = v.Raw
			str = v.Value
		case *parse.MultilineStringLiteral:
			encoded = v.Raw
			str = v.Value
		default:
			continue
		}

		err := analyzeInterpolationsInString(str, encoded, attr.Span)
		if err != nil {
			criticalErr = err
			return
		}
	}

	//Analyze client-side interpolations in markup text nodes.

	for _, child := range sourceNode.Node.Children {
		text, ok := child.(*parse.MarkupText)

		if !ok {
			continue
		}

		err := analyzeInterpolationsInString(text.Value, text.Raw, text.Span)
		if err != nil {
			criticalErr = err
			return
		}
	}

	return
}
