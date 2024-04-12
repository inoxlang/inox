package core

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// A MarkupPattern does not depend on a specific markup language. It tests implementations of the MarkupNode interface.
type MarkupPattern struct {
	topElement *MarkupPatternElement
	NotCallablePatternMixin
}

func NewMarkupPatternFromExpression(node *parse.MarkupPatternExpression, bridge StateBridge) (*MarkupPattern, error) {
	elem, err := newMarkupPatternElementFromNode(node.Element, bridge)
	if err != nil {
		return nil, err
	}
	return NewMarkupPattern(elem), nil
}

func NewMarkupPattern(topElement *MarkupPatternElement) *MarkupPattern {
	return &MarkupPattern{
		topElement: topElement,
	}
}

func newMarkupPatternElementFromNode(node *parse.MarkupPatternElement, bridge StateBridge) (*MarkupPatternElement, error) {

	attributes := map[string]StringPattern{}
	var children []MarkupPatternNode

	//Evaluate attributes.

	for _, attr := range node.Opening.Attributes {
		patternAttribute := attr.(*parse.MarkupPatternAttribute)
		name := patternAttribute.GetName()

		var stringPattern StringPattern

		if patternAttribute.Type != nil {
			val, err := evalNodeInMarkupPattern(patternAttribute.Type, bridge)
			if err != nil {
				return nil, err
			}

			switch val := val.(type) {
			case Pattern:
				strPattern, ok := val.StringPattern()
				if !ok {
					return nil, fmt.Errorf("pattern provided for the attribute '%s' does not have a corresponding string pattern", name)
				}
				stringPattern = strPattern
			case Bool:
				stringPattern = FALSE_STRING_PATTERN
				if val {
					stringPattern = TRUE_STRING_PATTERN
				}
			case Int:
				stringified := String(strconv.FormatInt(int64(val), 10))
				stringPattern = NewExactStringPattern(stringified)
			case GoString:
				stringPattern = NewExactStringPattern(String(val.UnderlyingString()))
			case Rune:
				stringPattern = NewExactStringPattern(String(val))
			default:
				//Note: floats are not supported because they do not have a unique representation.
				return nil, fmt.Errorf("unexpected value of type %T was found for the attribute '%s', a pattern was expected", val, name)
			}

		} else {
			stringPattern = ANY_STRING_REGEX_PATTERN
		}

		attributes[name] = stringPattern
	}

	//Get children nodes.

	if node.RawElementContent != "" {
		children = append(children, utils.Must(NewMarkupPatternConstText(node.RawElementContent)))
	} else {
		for _, child := range node.Children {
			switch child := child.(type) {
			case *parse.MarkupText:
				value := strings.TrimSpace(child.Value)
				if value != "" {
					children = append(children, utils.Must(NewMarkupPatternConstText(value)))
				}
			case *parse.MarkupPatternWildcard:
				children = append(children, &MarkupPatternWildcard{})
			case *parse.MarkupPatternElement:
				childElement, err := newMarkupPatternElementFromNode(child, bridge)
				if err != nil {
					return nil, err
				}
				children = append(children, childElement)
			case *parse.MarkupPatternInterpolation:
				val, err := evalNodeInMarkupPattern(child.Expr, bridge)
				if err != nil {
					return nil, err
				}

				switch val := val.(type) {
				case *MarkupPattern:
					children = append(children, val.topElement)
				case Bool:
					text := "false"
					if val {
						text = "true"
					}
					children = append(children, utils.Must(NewMarkupPatternConstText(text)))
				case Int:
					stringified := strconv.FormatInt(int64(val), 10)
					children = append(children, utils.Must(NewMarkupPatternConstText(stringified)))
				case GoString:
					text := strings.TrimSpace(val.UnderlyingString())
					if text != "" {
						children = append(children, utils.Must(NewMarkupPatternConstText(text)))
					}
				case Rune:
					children = append(children, utils.Must(NewMarkupPatternConstText(string(val))))
				default:
					return nil, fmt.Errorf(
						"unexpected value of type %T was found for the attribute '%s', a markup pattern or a value convertible to text was expected",
						val, val)
				}
			}
		}
	}

	return NewMarkupPatternElement(NewMarkupPatternElementParameters{
		TagName:    node.Opening.GetName(),
		Quantifier: node.Opening.Quantifier,
		Attributes: attributes,
		Children:   children,
	}), nil
}

type NewMarkupPatternElementParameters struct {
	TagName    string
	Quantifier parse.MarkupPatternElementQuantifier
	Attributes map[string]StringPattern
	Children   []MarkupPatternNode
}

func NewMarkupPatternElement(params NewMarkupPatternElementParameters) *MarkupPatternElement {
	return &MarkupPatternElement{
		tagName:    params.TagName,
		quantifier: params.Quantifier,
		attributes: params.Attributes,
		children:   params.Children,
	}
}

func evalNodeInMarkupPattern(e parse.Node, bridge StateBridge) (Value, error) {
	switch e := e.(type) {
	case *parse.MemberExpression:
		left, err := evalNodeInMarkupPattern(e.Left, bridge)
		if err != nil {
			return nil, err
		}
		return left.(IProps).Prop(bridge.Context, e.PropertyName.Name), nil
	case *parse.Variable:
		value, ok := bridge.GetVariableValue(e.Name)
		if !ok {
			return nil, fmt.Errorf("variable %s is not defined", e.Name)
		}
		return value, nil
	case *parse.PatternIdentifierLiteral:
		return bridge.Context.ResolveNamedPattern(e.Name), nil
	case *parse.PatternNamespaceMemberExpression:
		namespace := bridge.Context.ResolvePatternNamespace(e.Namespace.Name)
		pattern, ok := namespace.Patterns[e.MemberName.Name]
		if !ok {
			return nil, fmt.Errorf("pattern namespace %s does not have a member %s", e.Namespace.Name, e.MemberName.Name)
		}
		return pattern, nil
	case *parse.IdentifierLiteral:
		return nil, fmt.Errorf("cannot evaluate node of type %T in markup pattern: not supported", e)
	case parse.SimpleValueLiteral:
		return EvalSimpleValueLiteral(e, nil)
	case *parse.MarkupPatternExpression:
		return NewMarkupPatternFromExpression(e, bridge)
	default:
		return nil, fmt.Errorf("cannot evaluate node of type %T in markup pattern: not supported", e)
	}
}

func (patt *MarkupPattern) Test(ctx *Context, v Value) bool {
	markupNode, ok := v.(MarkupNode)
	if !ok {
		return false
	}

	immutableNode, pool := markupNode.ImmutableMarkupNode()

	if pool != nil {
		defer pool.Put(immutableNode)
	}

	return patt.topElement.Test(ctx, immutableNode, pool, false, false)
}

func (patt *MarkupPattern) StringPattern() (StringPattern, bool) {
	return nil, false
}

// Pattern node implementations.

type MarkupPatternNode interface {
	Test(ctx *Context, node ImmutableMarkupNode, pool *sync.Pool, isPrevChildPatternWildcard, isNextChildPatternWildcard bool) bool
}

type MarkupPatternElement struct {
	tagName    string
	quantifier parse.MarkupPatternElementQuantifier
	attributes map[string]StringPattern
	children   []MarkupPatternNode
}

func (patt *MarkupPatternElement) Test(ctx *Context, node ImmutableMarkupNode, pool *sync.Pool, _, _ bool) bool {

	if ctx.IsDone() {
		panic(ctx.Err())
	}

	if !node.IsMarkupElement() {
		return false
	}

	tagName, _ := node.MarkupTagName()

	if tagName != patt.tagName {
		return false
	}

	//Check attributes.

	for name, pattern := range patt.attributes {
		value, ok := node.MarkupAttributeValue(name)
		if !ok {
			return false
		}

		if !pattern.Test(ctx, String(value)) {
			return false
		}
	}

	//Check child nodes.

	return patt.testChildren(ctx, node, pool)
}

func (patt *MarkupPatternElement) testChildren(ctx *Context, node ImmutableMarkupNode, pool *sync.Pool) bool {
	childCount := node.MarkupChildNodeCount()
	childIndex := 0
	var child ImmutableMarkupNode

	var encounteredChildren []ImmutableMarkupNode

	if pool != nil {
		defer func() {
			for _, child := range encounteredChildren {
				pool.Put(child)
			}
		}()
	}

	eatWhitespaceTextNodes := func() {
		if child == nil {
			return
		}
		for {
			text, ok := child.MarkupText()
			if ok && strings.TrimSpace(text) == "" {
				//Move to next child.

				childIndex++
				if childIndex == childCount {
					child = nil
					return
				} else {
					child = node.MarkupChild(childIndex)
					encounteredChildren = append(encounteredChildren, child)
					continue
				}
			}
			return
		}
	}

	if childCount > 0 {
		childIndex = 0
		child = node.MarkupChild(childIndex)
		encounteredChildren = append(encounteredChildren, child)

		eatWhitespaceTextNodes()
		if len(patt.children) == 0 {
			ok := childIndex == childCount //only text nodes with white space.
			return ok
		}
	}

	isPrevChildPatternWildcard := false

	for patternIndex, childPattern := range patt.children {

		//Get the next child pattern.

		var nextChildPattern MarkupPatternNode
		isNextChildPatternWildcard := false

		if patternIndex+1 < len(patt.children) {
			nextChildPattern = patt.children[patternIndex+1]
			isNextChildPatternWildcard = utils.Implements[*MarkupPatternWildcard](nextChildPattern)
		}

		switch childPattern := childPattern.(type) {

		//Element with an (optional) quantifier.
		case *MarkupPatternElement:
			matchCount := 0

		match_elements:
			for child != nil {
				eatWhitespaceTextNodes()

				if child == nil { //no remaining children
					break match_elements
				}

				if childPattern.Test(ctx, child, pool, isPrevChildPatternWildcard, isNextChildPatternWildcard) {
					//Move to next child.
					childIndex++
					if childIndex == childCount {
						child = nil
					} else {
						child = node.MarkupChild(childIndex)
						encounteredChildren = append(encounteredChildren, child)
					}

					//If the quantifier falls in the category 'at most one', we exit the loop.
					switch childPattern.quantifier {
					case parse.OneMarkupElement, parse.OptionalMarkupElement:
						matchCount = 1
						break match_elements
					}
					matchCount++
				} else {
					break match_elements
				}
			}
			switch childPattern.quantifier {
			case parse.OneMarkupElement, parse.OneOrMoreMarkupElements:
				if matchCount == 0 {
					return false
				}
			default:
				//else: ok
			}
		//Text
		case *MarkupPatternConstText:
			if child == nil {
				return false
			}

			if !childPattern.Test(ctx, child, pool, isPrevChildPatternWildcard, isNextChildPatternWildcard) {
				return false
			}
			//Move to next child.
			childIndex++

			if childIndex == childCount {
				child = nil
			} else {
				child = node.MarkupChild(childIndex)
				encounteredChildren = append(encounteredChildren, child)
			}
		//Lazy wildcard
		case *MarkupPatternWildcard:

			if nextChildPattern == nil {
				//If a '*' wildcard is the last pattern we do not need to check the remaining children.
				childIndex = childCount
				goto children_checked
			}

			_isPrevChildPatternWildcard := true
			_isNextChildPatternWildcard := patternIndex+2 < len(patt.children) &&
				utils.Implements[*MarkupPatternWildcard](patt.children[patternIndex+2])

			for child != nil {
				eatWhitespaceTextNodes()

				//Stop when the next pattern matches the child.

				if nextChildPattern.Test(ctx, child, pool, _isPrevChildPatternWildcard, _isNextChildPatternWildcard) {
					break
				}

				//Move to next child.
				childIndex++

				if childIndex == childCount {
					child = nil
				} else {
					child = node.MarkupChild(childIndex)
					encounteredChildren = append(encounteredChildren, child)
				}
			}
		}
		isPrevChildPatternWildcard = utils.Implements[*parse.MarkupWildcard](childPattern)
	}

	eatWhitespaceTextNodes()

	//Check that there no remaining children.
	if childIndex != childCount {
		return false
	}

children_checked:
	return true
}

type MarkupPatternConstText struct {
	value string //trimmed
}

func NewMarkupPatternConstText(value string) (*MarkupPatternConstText, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, fmt.Errorf("text only contains white space")
	}

	return &MarkupPatternConstText{
		value: value,
	}, nil
}

func (patt *MarkupPatternConstText) Test(
	ctx *Context, node ImmutableMarkupNode, _ *sync.Pool,
	isPrevChildPatternWildcard, isNextChildPatternWildcard bool,
) bool {

	text, ok := node.MarkupText()
	if !ok {
		return false
	}
	text = strings.TrimSpace(text)

	switch {
	case isPrevChildPatternWildcard && isNextChildPatternWildcard:
		// .* pattern text .*
		return strings.Contains(text, patt.value)
	case isPrevChildPatternWildcard:
		// .* pattern text
		return strings.HasSuffix(text, patt.value)
	case isNextChildPatternWildcard:
		// pattern text .*
		return strings.HasPrefix(text, patt.value)
	default:
		return text == patt.value
	}
}

type MarkupPatternWildcard struct {
}

func (patt *MarkupPatternWildcard) Test(
	ctx *Context, node ImmutableMarkupNode, _ *sync.Pool,
	isPrevChildPatternWildcard, isNextChildPatternWildcard bool,
) bool {
	return true
}
