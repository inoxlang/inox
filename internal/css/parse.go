package css

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/tdewolff/parse/v2"

	"github.com/tdewolff/parse/v2/css"
)

func ParseString(ctx context.Context, s string) (Node, error) {
	return _parse(ctx, parse.NewInputString(s))
}

func ParseRead(ctx context.Context, r io.Reader) (Node, error) {
	return _parse(ctx, parse.NewInput(r))
}

func _parse(ctx context.Context, input *parse.Input) (Node, error) {
	parser := css.NewParser(input, false)

	var stack []Node

	stack = append(stack, Node{
		Type: Stylesheet,
	})

	current := 0
	parent := -1
	noCheckFuel := 11 //check the context every time we run out of 'no check fuel'.

	for {
		noCheckFuel--
		if noCheckFuel == 0 {
			noCheckFuel = 10
			if utils.IsContextDone(ctx) {
				return Node{}, ctx.Err()
			}
		}

		gt, _, data := parser.Next()

		switch gt {
		case css.CommentGrammar:
			comment := Node{
				Type: Comment,
				Data: string(data),
			}
			stack[current].Children = append(stack[current].Children, comment)
		case css.AtRuleGrammar:
			atRule := Node{Type: AtRule, Data: string(data)}

			err := makeNodesFromTokens(parser.Values(), &atRule)
			if err != nil {
				return Node{}, err
			}

			stack[current].Children = append(stack[current].Children, atRule)
		case css.BeginAtRuleGrammar:
			stack = append(stack, Node{Type: AtRule, Data: string(data)})
			parent++
			current++

			stack[current].Children = append(stack[current].Children, Node{
				Type: MediaQuery,
			})

			mediaQuery := &stack[current].Children[0]

			err := makeNodesFromTokens(parser.Values(), mediaQuery)
			if err != nil {
				return Node{}, err
			}
		case css.BeginRulesetGrammar:
			stack = append(stack, Node{Type: Ruleset})
			current++
			parent++

			stack[current].Children = append(stack[current].Children, Node{
				Type: Selector,
			})

			selector := &stack[current].Children[0]

			err := makeNodesFromTokens(parser.Values(), selector)
			if err != nil {
				return Node{}, err
			}
		case css.DeclarationGrammar:
			stack[current].Children = append(stack[current].Children, Node{
				Type: Declaration,
			})

			declaration := &stack[current].Children[len(stack[current].Children)-1]
			declaration.Data = string(data)

			err := makeNodesFromTokens(parser.Values(), declaration)
			if err != nil {
				return Node{}, err
			}
		case css.EndAtRuleGrammar, css.EndRulesetGrammar:
			prev := stack[current]
			parent--
			current--
			stack = stack[:len(stack)-1]

			stack[current].Children = append(stack[current].Children, prev)
		}

		// fmt.Println(" <" + gt.String() + "> ")

		// for _, val := range parser.Values() {
		// 	fmt.Println(" (" + val.String() + ") ")
		// }

		if gt == css.ErrorGrammar {
			break
		}
	}

	return stack[0], nil
}

func makeNodesFromTokens(tokens []css.Token, parentNode *Node) error {
	i := 0

	return _makeNodesFromTokens(tokens, parentNode, &i, nil)
}

func _makeNodesFromTokens(tokens []css.Token, parentNode *Node, i *int, stop func(t css.Token) bool) error {

	precededByDot := false
	leadingSpace := true

	inMediaFeature := false
	var mediaFeature Node

	if parentNode.Type == MediaQuery {
		for *i < len(tokens) {

			t := tokens[*i]
			*i = (*i + 1)

			switch t.TokenType {
			case css.LeftParenthesisToken:
				inMediaFeature = true
				mediaFeature.Type = MediaFeature
			case css.RightParenthesisToken:
				inMediaFeature = false
				parentNode.Children = append(parentNode.Children, mediaFeature)
				mediaFeature = Node{}
			case css.IdentToken:
				if inMediaFeature && mediaFeature.Data == "" {
					mediaFeature.Data = string(t.Data)
				} else {
					parentNode.Children = append(parentNode.Children, Node{
						Type: Ident,
						Data: string(t.Data),
					})
				}
			case css.ColonToken, css.WhitespaceToken:
				//ignore
			default:
				node, _ := makeSimpleNodeFromToken(t, precededByDot)

				if inMediaFeature {
					mediaFeature.Children = append(mediaFeature.Children, node)
				} else {
					parentNode.Children = append(parentNode.Children, node)
				}
			}
		}

		return nil
	}

	//other node types

	for *i < len(tokens) {

		t := tokens[*i]
		*i = (*i + 1)

		if stop != nil && stop(t) {
			return nil
		}

		if t.TokenType == css.WhitespaceToken && leadingSpace {
			continue
		}

		leadingSpace = false

		if t.TokenType == css.DelimToken && len(t.Data) == 1 && t.Data[0] == '.' {
			precededByDot = true
			continue
		}

		if precededByDot && t.TokenType != css.IdentToken {
			return fmt.Errorf("delim '.' not followed by an identifier")
		}

		switch t.TokenType {
		case css.FunctionToken:
			functionCall := Node{
				Type: FunctionCall,
				Data: strings.TrimSuffix(string(t.Data), "("),
			}

			err := _makeNodesFromTokens(tokens, &functionCall, i, func(t css.Token) bool {
				return t.TokenType == css.RightParenthesisToken
			})

			parentNode.Children = append(parentNode.Children, functionCall)

			if err != nil {
				return err
			}
		case css.LeftParenthesisToken:
			expr := Node{
				Type: ParenthesizedExpr,
			}
			err := _makeNodesFromTokens(tokens, &expr, i, func(t css.Token) bool {
				return t.TokenType == css.RightParenthesisToken
			})

			if err != nil {
				return err
			}

			parentNode.Children = append(parentNode.Children, expr)
		case css.LeftBracketToken:
			expr := Node{
				Type: AttributeSelector,
			}
			err := _makeNodesFromTokens(tokens, &expr, i, func(t css.Token) bool {
				return t.TokenType == css.RightBracketToken
			})
			if err != nil {
				return err
			}
			parentNode.Children = append(parentNode.Children, expr)
		default:
			node, isSignificant := makeSimpleNodeFromToken(t, precededByDot)
			if isSignificant {
				parentNode.Children = append(parentNode.Children, node)
			}
			precededByDot = false
		}
	}
	return nil
}

func makeSimpleNodeFromToken(t css.Token, precededByDot bool) (n Node, significant bool) {

	if precededByDot && t.TokenType != css.IdentToken {
		panic(fmt.Errorf("onlt identifiers can be preceded by a dot"))
	}

	switch t.TokenType {
	case css.NumberToken:
		n.Type = Number
	case css.DimensionToken:
		n.Type = Dimension
	case css.IdentToken:
		if precededByDot {
			n.Type = ClassName
		} else {
			n.Type = Ident
		}
	case css.HashToken:
		n.Type = Hash
	case css.StringToken:
		n.Type = String
	case css.BadStringToken:
		n.Type = String
		n.Error = true
	case css.URLToken:
		n.Type = URL
	case css.BadURLToken:
		n.Type = URL
		n.Error = true
	case css.PercentageToken:
		n.Type = Percentage
	case css.UnicodeRangeToken:
		n.Type = UnicodeRange
	case css.IncludeMatchToken, css.DashMatchToken, css.SuffixMatchToken, css.SubstringMatchToken:
		n.Type = MatchOperator
	case css.CustomPropertyNameToken:
		n.Type = CustomPropertyName
	case css.CustomPropertyValueToken:
		n.Type = CustomPropertyName
	case css.WhitespaceToken:
		n.Type = Whitespace
	default:
		return Node{}, false
	}

	n.Data = string(t.Data)
	significant = true
	return
}
