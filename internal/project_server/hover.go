package project_server

import (
	"bufio"
	"bytes"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/help_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func getHoverContent(fpath string, line, column int32, handlingCtx *core.Context, session *jsonrpc.Session) (*defines.Hover, error) {
	state, mod, ok := prepareSourceFile(fpath, handlingCtx, session)
	if !ok {
		return &defines.Hover{}, nil
	}

	if state == nil || state.SymbolicData == nil {
		logs.Println("no data")
		return &defines.Hover{}, nil
	}

	span := mod.MainChunk.GetLineColumnSingeCharSpan(line, column)
	foundNode, ancestors, ok := mod.MainChunk.GetNodeAndChainAtSpan(span)

	if !ok || foundNode == nil {
		logs.Println("no data")
		return &defines.Hover{}, nil
	}

	//help about tag or attribute
	xmlElementInfo, hasXmlElementInfo := getXmlElementInfo(foundNode, ancestors)

	mostSpecificVal, ok := state.SymbolicData.GetMostSpecificNodeValue(foundNode)
	var lessSpecificVal symbolic.SymbolicValue
	if !ok {
		if hasXmlElementInfo {
			return &defines.Hover{
				Contents: defines.MarkupContent{
					Kind:  defines.MarkupKindMarkdown,
					Value: xmlElementInfo,
				},
			}, nil
		}

		logs.Println("no data")
		return &defines.Hover{}, nil
	}

	buff := &bytes.Buffer{}
	w := bufio.NewWriterSize(buff, 1000)
	var stringified string
	{
		utils.PanicIfErr(symbolic.PrettyPrint(mostSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
		var ok bool
		lessSpecificVal, ok = state.SymbolicData.GetLessSpecificNodeValue(foundNode)
		if ok {
			w.Write(utils.StringAsBytes("\n\n# less specific\n"))
			utils.PanicIfErr(symbolic.PrettyPrint(lessSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
		}

		w.Flush()
		stringified = strings.ReplaceAll(buff.String(), "\n\r", "\n")
		logs.Println(stringified)
	}

	//help for most specific & less specific values
	var helpMessage string
	{
		val := mostSpecificVal
		for {
			switch val := val.(type) {
			case *symbolic.GoFunction:
				text, ok := help_ns.HelpForSymbolicGoFunc(val, help_ns.HelpMessageConfig{Format: help_ns.MarkdownFormat})
				if ok {
					helpMessage = "\n-----\n" + strings.ReplaceAll(text, "\n\r", "\n")
				}
			}
			if helpMessage == "" && val == mostSpecificVal && lessSpecificVal != nil {
				val = lessSpecificVal
				continue
			}
			break
		}
	}

	if hasXmlElementInfo {
		helpMessage += "\n\n" + xmlElementInfo
	}

	codeBlock := ""
	if stringified != "" {
		codeBlock = "```inox\n" + stringified + "\n```"
	}

	return &defines.Hover{
		Contents: defines.MarkupContent{
			Kind:  defines.MarkupKindMarkdown,
			Value: codeBlock + helpMessage,
		},
	}, nil
}

func getXmlElementInfo(node parse.Node, ancestors []parse.Node) (string, bool) {
	if len(ancestors) < 3 {
		return "", false
	}

	ident, ok := node.(*parse.IdentifierLiteral)
	if !ok {
		return "", false
	}

	var (
		attribute   *parse.XMLAttribute
		openingElem *parse.XMLOpeningElement
		parent      parse.Node
		xmlExpr     *parse.XMLExpression
		tagIdent    *parse.IdentifierLiteral
	)

	parent = ancestors[len(ancestors)-1]
	attribute, ok = parent.(*parse.XMLAttribute)
	if ok {
		openingElem, ok = ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
		if !ok { //invalid state
			return "", false
		}
		tagIdent, ok = openingElem.Name.(*parse.IdentifierLiteral)
		if !ok { //parsing error
			return "", false
		}
	} else {
		openingElem, ok = parent.(*parse.XMLOpeningElement)
		if !ok { //invalid state
			return "", false
		}

		if ident != openingElem.Name {
			return "", false
		}
		tagIdent = ident
	}

	e, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return "", false
	}
	xmlExpr = e

	namespace, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return "", false
	}

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":

		if parent == openingElem {
			tagData, ok := html_ns.GetTagData(tagIdent.Name)
			if ok {
				return tagData.DescriptionContent(), true
			}
		} else if parent == attribute {

			attributes, ok := html_ns.GetAllTagAttributes(tagIdent.Name)
			if !ok {
				break
			}

			attrName := ident.Name

			for _, attr := range attributes {
				if attr.Name == attrName {
					return attr.DescriptionContent(), true
				}

			}
		}

	}

	return "", false
}
