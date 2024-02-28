package hsparse

// import (
// 	"github.com/inoxlang/inox/internal/hyperscript/hscode"
// )

// type parser struct {
// 	grammar             map[hscode.NodeType]parseRule
// 	features            map[string]parseRule
// 	commands            map[string]parseRule
// 	leafExpressions     []hscode.NodeType
// 	indirectExpressions []hscode.NodeType
// 	possessivesDisabled bool
// }

// type parseRule func(parser *parser, token *tokens, root *hscode.Node) hscode.Node

// func newParser() *parser {

// 	p := &parser{
// 		possessivesDisabled: false,
// 		grammar:             map[hscode.NodeType]parseRule{},
// 		features:            map[string]parseRule{},
// 		commands:            map[string]parseRule{},
// 	}

// 	/* ============================================================================================ */
// 	/* Core hyperscript Grammar Elements                                                            */
// 	/* ============================================================================================ */
// 	p.addGrammarElement("feature", func(p *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.matchOpToken("(").IsNotZero() {
// 			featureElement := p.requireElement("feature", tokens, "", nil)
// 			tokens.requireOpToken(")")
// 			return featureElement
// 		}

// 		featureDefinition := p.features[tokens.currentToken().Value]
// 		if featureDefinition != nil {
// 			return featureDefinition(p, tokens, nil)
// 		}

// 		return hscode.Node{}
// 	})

// 	p.addGrammarElement("command", func(p *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.matchOpToken("(").IsNotZero() {
// 			commandElement := p.requireElement("command", tokens, "", nil)
// 			tokens.requireOpToken(")")
// 			return commandElement
// 		}

// 		var commandDefinition = p.commands[tokens.currentToken().Value]
// 		var commandElement hscode.Node
// 		if commandDefinition != nil {
// 			commandElement = commandDefinition(p, tokens, nil)
// 		} else if tokens.currentToken().Type == "IDENTIFIER" {
// 			commandElement = p.parseElement("pseudoCommand", tokens, nil)
// 		}
// 		if commandElement.IsNotZero() {
// 			return p.parseElement("indirectStatement", tokens, &commandElement)
// 		}

// 		return commandElement
// 	})

// 	p.addGrammarElement("commandList", func(p *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		if tokens.hasMore() {
// 			var cmd = p.parseElement("command", tokens, nil)
// 			if cmd.IsNotZero() {
// 				tokens.matchToken("then", "")
// 				next := p.parseElement("commandList", tokens, nil)
// 				if next.IsNotZero() {
// 					cmd.Next = &next
// 				}
// 				return cmd
// 			}
// 		}
// 		return hscode.Node{
// 			Type: "emptyCommandListCommand",
// 		}
// 	})

// 	p.addGrammarElement("leaf", func(p *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		result := p.parseAnyOf(p.leafExpressions, tokens)
// 		// symbol is last so it doesn't consume any constants
// 		if result.IsZero() {
// 			return p.parseElement("symbol", tokens, nil)
// 		}

// 		return result
// 	})

// 	p.addGrammarElement("indirectExpression", func(p *parser, tokens *tokens, root *hscode.Node) hscode.Node {
// 		for i := 0; i < len(p.indirectExpressions); i++ {
// 			indirect := p.indirectExpressions[i]
// 			root.EndToken = tokens.lastMatch()
// 			var result = p.parseElement(indirect, tokens, root)
// 			if result.IsNotZero() {
// 				return result
// 			}
// 		}
// 		return *root
// 	})

// 	p.addGrammarElement("indirectStatement", func(p *parser, tokens *tokens, root *hscode.Node) hscode.Node {
// 		if tokens.matchToken("unless", "").IsNotZero() {
// 			root.EndToken = tokens.lastMatch()
// 			var conditional = p.requireElement("expression", tokens, "", nil)
// 			unless := hscode.Node{
// 				Type: "unlessStatementModifier",
// 				Args: []hscode.Node{conditional},
// 			}
// 			//root.parent = unless
// 			return unless
// 		}
// 		return *root

// 	})

// 	p.addGrammarElement("primaryExpression", func(p *parser, tokens *tokens, _ *hscode.Node) hscode.Node {
// 		leaf := p.parseElement("leaf", tokens, nil)
// 		if leaf.IsNotZero() {
// 			return p.parseElement("indirectExpression", tokens, &leaf)
// 		}

// 		msg := "Unexpected value: " + tokens.currentToken().Value

// 		panic(&hscode.ParsingError{
// 			Message:        msg,
// 			MessageAtToken: msg,
// 			Token:          tokens.currentToken(),
// 			Tokens:         tokens.initial,
// 		})
// 	})

// 	return p
// }

// func (p *parser) parseHyperScript(tokens *tokens) hscode.Node {
// 	var result = p.parseElement("hyperscript", tokens, nil)
// 	if tokens.hasMore() {
// 		p.throwParsingError(tokens, "")
// 	}
// 	if result.IsNotZero() {
// 		return result
// 	}

// 	return hscode.Node{}
// }

// func (p *parser) addGrammarElement(name hscode.NodeType, parseRule parseRule) {
// 	p.grammar[name] = parseRule
// }

// func (p *parser) addCommand(keyword string, definition parseRule) {
// 	commandGrammarType := hscode.NodeType(keyword + "Command")
// 	commandDefinitionWrapper := func(parser *parser, tokens *tokens, root *hscode.Node) hscode.Node {
// 		commandElement := definition(parser, tokens, nil)
// 		if commandElement.IsNotZero() {
// 			commandElement.Type = commandGrammarType
// 			return commandElement
// 		}
// 		return hscode.Node{}
// 	}
// 	p.grammar[commandGrammarType] = commandDefinitionWrapper
// 	p.commands[keyword] = commandDefinitionWrapper
// }

// func (p *parser) addFeature(keyword string, definition parseRule) {
// 	var featureGrammarType = keyword + "Feature"

// 	featureDefinitionWrapper := func(parser *parser, tokens *tokens, root *hscode.Node) hscode.Node {
// 		var featureElement = definition(parser, tokens, nil)
// 		if featureElement.IsNotZero() {
// 			featureElement.IsFeature = true
// 			featureElement.Keyword = keyword
// 			featureElement.Type = hscode.NodeType(featureGrammarType)
// 			return featureElement
// 		}
// 		return hscode.Node{}
// 	}
// 	p.grammar[hscode.NodeType(featureGrammarType)] = featureDefinitionWrapper
// 	p.features[keyword] = featureDefinitionWrapper
// }

// func (p *parser) parseElement(typ hscode.NodeType, tokens *tokens, root *hscode.Node) hscode.Node {
// 	var elementDefinition = p.grammar[typ]
// 	if elementDefinition != nil {
// 		start := tokens.currentToken()
// 		parseElement := elementDefinition(p, tokens, root)
// 		if parseElement.IsNotZero() {
// 			p.initElement(&parseElement, start, tokens)
// 			if parseElement.EndToken.IsZero() {
// 				parseElement.EndToken = tokens.lastMatch()
// 			}
// 			var root = parseElement.Root
// 			for root != nil {
// 				p.initElement(root, start, tokens)
// 				root = root.Root
// 			}
// 		}
// 		return parseElement
// 	}

// 	return hscode.Node{}
// }

// func (p *parser) initElement(e *hscode.Node, start hscode.Token, tokens *tokens) {
// 	e.StartToken = start
// 	//e.sourceFor = Tokens.sourceFor
// 	//e.lineFor = Tokens.lineFor
// 	//e.programSource = tokens.source
// }

// func (p *parser) requireElement(typ hscode.NodeType, tokens *tokens, message string, root *hscode.Node) hscode.Node {
// 	var result = p.parseElement(typ, tokens, root)
// 	if result.IsZero() {
// 		msg := message
// 		if message == "" {
// 			message = "Expected " + string(typ)
// 		}
// 		panic(&hscode.ParsingError{
// 			Message:        message,
// 			MessageAtToken: msg,
// 			Token:          tokens.currentToken(),
// 			Tokens:         tokens.initial,
// 		})
// 	}
// 	// @ts-ignore
// 	return result
// }

// func (p *parser) parseAnyOf(types []hscode.NodeType, tokens *tokens) hscode.Node {
// 	for i := 0; i < len(types); i++ {
// 		typ := types[i]
// 		var expression = p.parseElement(typ, tokens, nil)
// 		if expression.IsNotZero() {
// 			return expression
// 		}
// 	}
// 	return hscode.Node{}
// }

// func (p *parser) addLeafExpression(name hscode.NodeType, definition parseRule) {
// 	p.leafExpressions = append(p.leafExpressions)
// 	p.addGrammarElement(name, definition)
// }

// func (p *parser) addIndirectExpression(name hscode.NodeType, definition parseRule) {
// 	p.indirectExpressions = append(p.indirectExpressions, name)
// 	p.addGrammarElement(name, definition)
// }

// func (p *parser) throwParsingError(tokens *tokens, message string) {
// 	token := tokens.currentToken()

// 	messageAtToken := message
// 	completeMessage := message

// 	if messageAtToken == "" {
// 		message = "Unexpected Token : " + token.Value
// 		completeMessage = messageAtToken + "\n\n" + createParserContext(tokens)
// 	}

// 	panic(&hscode.ParsingError{
// 		Message:        completeMessage,
// 		MessageAtToken: messageAtToken,
// 		Token:          token,
// 		Tokens:         tokens.initial,
// 	})
// }

// func (p *parser) commandStart(token hscode.Token) parseRule {
// 	return p.commands[token.Value]
// }

// func (p *parser) featureStart(token hscode.Token) parseRule {
// 	return p.features[token.Value]
// }

// func (p *parser) commandBoundary(token hscode.Token) bool {
// 	if token.Value == "end" ||
// 		token.Value == "then" ||
// 		token.Value == "else" ||
// 		token.Value == "otherwise" ||
// 		token.Value == ")" ||
// 		p.commandStart(token) != nil ||
// 		p.featureStart(token) != nil ||
// 		token.Type == "EOF" {
// 		return true
// 	}
// 	return false
// }

// // Elements in the returned slice are Node with the property .PseudoStringNodeValue potentiall set.
// func (p *parser) parseStringTemplate(tokens *tokens) []hscode.Node {
// 	var returnArr = []hscode.Node{{PseudoStringNodeValue: ""}}

// 	step := func() {
// 		returnArr = append(returnArr, hscode.Node{PseudoStringNodeValue: tokens.lastWhitespace()})
// 		if tokens.currentToken().Value == "$" {
// 			tokens.consumeToken()
// 			var startingBrace = tokens.matchOpToken("{")
// 			returnArr = append(returnArr, p.requireElement("expression", tokens, "", nil))
// 			if startingBrace.IsNotZero() {
// 				tokens.requireOpToken("}")
// 			}
// 			returnArr = append(returnArr, hscode.Node{PseudoStringNodeValue: ""})
// 		} else if tokens.currentToken().Value == "\\" {
// 			tokens.consumeToken() // skip next
// 			tokens.consumeToken()
// 		} else {
// 			var token = tokens.consumeToken()
// 			value := ""
// 			if token.IsNotZero() {
// 				value = token.Value
// 			}
// 			node := returnArr[len(returnArr)-1]
// 			node.PseudoStringNodeValue += value
// 			returnArr[len(returnArr)-1] = node
// 		}
// 	}

// 	step()

// 	for tokens.hasMore() {
// 		step()
// 	}
// 	returnArr = append(returnArr, hscode.Node{PseudoStringNodeValue: tokens.lastWhitespace()})
// 	return returnArr
// }

// func (p *parser) ensureTerminated(commandList *hscode.Node) {
// 	implicitReturn := hscode.Node{
// 		Type: "implicitReturn",
// 	}

// 	var end = commandList
// 	for end.Next != nil {
// 		end = end.Next
// 	}
// 	end.Next = &implicitReturn
// }

// func createParserContext(tokens *tokens) string {
// 	return "<parser context: TODO>"
// 	// var currentToken = tokens.currentToken()

// 	// var lines = strings.Split(tokens.sourceString, "\n")
// 	// var line int32
// 	// var offset int32

// 	// var contextLine string = lines[line]

// 	// if currentToken.IsNotZero() && currentToken.Line >= 1 {
// 	// 	line = currentToken.Line - 1
// 	// 	offset = currentToken.Column
// 	// } else {
// 	// 	line = len(lines) - 1
// 	// 	offset = len(contextLine) - 1
// 	// }
// 	// return contextLine + "\n" + strings.Repeat(" ", int(offset)) + "^^\n\n"
// }
