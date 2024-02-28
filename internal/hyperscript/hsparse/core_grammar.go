package hsparse

// import (
// 	"strconv"
// 	"strings"

// 	"github.com/inoxlang/inox/internal/hyperscript/hscode"
// 	"github.com/inoxlang/inox/internal/utils"
// )

// func hyperscriptCoreGrammar(p *parser) {
// 	p.addLeafExpression("parenthesized", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.matchOpToken("(").IsNotZero() {
// 			var follows = tokens.clearFollows()
// 			var expr hscode.Node
// 			defer func() {
// 				tokens.restoreFollows(follows)
// 			}()
// 			expr = parser.requireElement("expression", tokens, "", nil)
// 			tokens.requireOpToken(")")
// 			return expr
// 		}
// 		return hscode.Node{}
// 	})

// 	p.addLeafExpression("string", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var stringToken = tokens.matchTokenType("STRING")
// 		if stringToken.IsZero() {
// 			return hscode.Node{}
// 		}
// 		var rawValue string = stringToken.Value
// 		var args []hscode.Node
// 		if stringToken.Template {
// 			l := NewLexer()
// 			innerTokens, err := l.tokenize(rawValue, true)
// 			if err != nil {
// 				panic(err)
// 			}
// 			args = parser.parseStringTemplate(NewTokens(innerTokens, nil, []rune(rawValue), rawValue))
// 		} else {
// 			args = []hscode.Node{}
// 		}
// 		return hscode.Node{
// 			Type:  "string",
// 			Token: &stringToken,
// 			Args:  args,
// 		}
// 	})

// 	p.addGrammarElement("nakedString", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.hasMore() {
// 			var tokenArr = tokens.consumeUntilWhitespace()
// 			tokens.matchTokenType("WHITESPACE")
// 			return hscode.Node{
// 				Type:   "nakedString",
// 				Tokens: tokenArr,
// 			}
// 		}
// 		return hscode.Node{}
// 	})

// 	p.addLeafExpression("number", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var number = tokens.matchTokenType("NUMBER")
// 		if number.IsZero() {
// 			return hscode.Node{}
// 		}
// 		var numberToken = number
// 		value, err := strconv.ParseFloat(number.Value, 64)
// 		if err != nil {
// 			panic(err)
// 		}
// 		return hscode.Node{
// 			Type:        "number",
// 			Value:       &value,
// 			NumberToken: &numberToken,
// 		}
// 	})

// 	p.addLeafExpression("idRef", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var elementId = tokens.matchTokenType("ID_REF")
// 		if elementId.IsZero() || elementId.Value == "" {
// 			return hscode.Node{}
// 		}
// 		var templateValue string
// 		var innerExpression hscode.Node
// 		if elementId.Template {
// 			templateValue = elementId.Value[2:]

// 			l := NewLexer()
// 			innerTokenList, err := l.tokenize(templateValue, true)
// 			if err != nil {
// 				panic(err)
// 			}
// 			innerTokens := NewTokens(innerTokenList, nil, []rune(templateValue), templateValue)

// 			innerExpression = parser.requireElement("expression", innerTokens, "", nil)
// 		} else {
// 			var value = elementId.Value[1:]
// 			return hscode.Node{
// 				Type:  "idRef",
// 				CSS:   elementId.Value,
// 				Value: value,
// 			}
// 		}
// 		return hscode.Node{
// 			Type: "idRefTemplate",
// 			Args: []hscode.Node{innerExpression},
// 		}
// 	})

// 	p.addLeafExpression("classRef", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var classRef = tokens.matchTokenType("CLASS_REF")
// 		if classRef.IsZero() || classRef.Value == "" {
// 			return hscode.Node{}
// 		}
// 		var templateValue string
// 		var innerExpression hscode.Node
// 		if classRef.Template {
// 			templateValue = classRef.Value[2:]
// 			l := NewLexer()
// 			innerTokenList, err := l.tokenize(templateValue, true)
// 			if err != nil {
// 				panic(err)
// 			}
// 			innerTokens := NewTokens(innerTokenList, nil, []rune(templateValue), templateValue)
// 			innerExpression = parser.requireElement("expression", innerTokens, "", nil)
// 		} else {
// 			var css = classRef.Value
// 			return hscode.Node{
// 				Type: "classRef",
// 				CSS:  css,
// 			}
// 		}
// 		return hscode.Node{
// 			Type: "classRefTemplate",
// 			Args: []hscode.Node{innerExpression},
// 		}
// 	})

// 	p.addLeafExpression("queryRef", func(parser *parser, tks *tokens, _ *hscode.Node) hscode.Node {
// 		var queryStart = tks.matchOpToken("<")
// 		if queryStart.IsZero() {
// 			return hscode.Node{}
// 		}
// 		var queryTokens = tks.consumeUntil("/", "")
// 		tks.requireOpToken("/")
// 		tks.requireOpToken(">")
// 		var queryValueParts = utils.MapSlice(queryTokens, func(t hscode.Token) string {
// 			if t.Type == "STRING" {
// 				return `"` + t.Value + `"`
// 			}
// 			return t.Value
// 		})
// 		queryValue := strings.Join(queryValueParts, "")

// 		//var template bool
// 		var innerTokens *tokens
// 		var args []hscode.Node
// 		if strings.Contains(queryValue, "$") {
// 			//template = true
// 			l := NewLexer()
// 			innerTokenList, err := l.tokenize(queryValue, true)
// 			if err != nil {
// 				panic(err)
// 			}
// 			innerTokens = NewTokens(innerTokenList, nil, []rune(queryValue), queryValue)
// 			args = parser.parseStringTemplate(innerTokens)
// 		}

// 		return hscode.Node{
// 			Type: "queryRef",
// 			CSS:  queryValue,
// 			Args: args,
// 		}
// 	})

// 	p.addLeafExpression("attributeRef", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var attributeRef = tokens.matchTokenType("ATTRIBUTE_REF")
// 		if attributeRef.IsZero() || attributeRef.Value == "" {
// 			return hscode.Node{}
// 		}
// 		var outerVal = attributeRef.Value
// 		var innerValue string
// 		if outerVal[0] == '[' {
// 			innerValue = outerVal[2 : len(outerVal)-1]
// 		} else {
// 			innerValue = outerVal[1:]
// 		}
// 		var css = "[" + innerValue + "]"
// 		var split = strings.Split(innerValue, "=")
// 		var name = split[0]
// 		var value = split[1]
// 		if value != "" {
// 			if value[0] == '"' {
// 				value = value[1 : len(value)-1]
// 			}
// 		}
// 		return hscode.Node{
// 			Type:  "attributeRef",
// 			Name:  name,
// 			CSS:   css,
// 			Value: value,
// 		}
// 	})

// 	p.addLeafExpression("styleRef", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		var styleRef = tokens.matchTokenType("STYLE_REF")
// 		if styleRef.IsZero() || styleRef.Value == "" {
// 			return hscode.Node{}
// 		}
// 		var styleProp = styleRef.Value[1:]
// 		if strings.HasPrefix(styleProp, "computed-") {
// 			styleProp = styleProp[len("computed-"):]
// 			return hscode.Node{
// 				Type: "computedStyleRef",
// 				Name: styleProp,
// 			}
// 		} else {
// 			return hscode.Node{
// 				Type: "styleRef",
// 				Name: styleProp,
// 			}
// 		}
// 	})

// 	p.addGrammarElement("objectKey", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		token := tokens.matchTokenType("STRING")
// 		if token.IsNotZero() {
// 			return hscode.Node{
// 				Type: "objectKey",
// 				Key:  token.Value,
// 			}
// 		} else if tokens.matchOpToken("[").IsNotZero() {
// 			var expr = parser.parseElement("expression", tokens, nil)
// 			tokens.requireOpToken("]")
// 			return hscode.Node{
// 				Type: "objectKey",
// 				Expr: &expr,
// 				Args: []hscode.Node{expr},
// 			}
// 		} else {
// 			var key string

// 			step := func() (contin bool) {
// 				token := tokens.matchTokenType("IDENTIFIER")
// 				if token.IsZero() {
// 					token = tokens.matchOpToken("-")
// 				}
// 				if token.IsZero() {
// 					return false
// 				}

// 				key += token.Value
// 				return true
// 			}

// 			for step() {
// 			}

// 			return hscode.Node{
// 				Type: "objectKey",
// 				Key:  key,
// 			}
// 		}
// 	})

// 	p.addLeafExpression("objectLiteral", func(parser *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.matchOpToken("{").IsZero() {
// 			return hscode.Node{}
// 		}
// 		var keyExpressions []hscode.Node
// 		var valueExpressions []hscode.Node
// 		if tokens.matchOpToken("}").IsZero() {
// 			for {
// 				var name = parser.requireElement("objectKey", tokens, "", nil)
// 				tokens.requireOpToken(":")
// 				var value = parser.requireElement("expression", tokens, "", nil)
// 				valueExpressions = append(valueExpressions, value)
// 				keyExpressions = append(keyExpressions, name)
// 				if tokens.matchOpToken(",").IsZero() {
// 					break
// 				}
// 			}
// 			tokens.requireOpToken("}")
// 		}
// 		return hscode.Node{
// 			Type: "objectLiteral",
// 			//TODO: Args: []hscode.Node{keyExpressions, valueExpressions},
// 		}
// 	})
// }
