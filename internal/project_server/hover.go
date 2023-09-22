package project_server

import (
	"bufio"
	"bytes"
	"fmt"
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
	state, _, chunk, ok := prepareSourceFileInDevMode(fpath, handlingCtx, session, true)
	if !ok {
		return &defines.Hover{}, nil
	}

	defer func() {
		go func() {
			defer utils.Recover()
			state.Ctx.CancelGracefully()
		}()
	}()

	if state == nil || state.SymbolicData == nil {
		logs.Println("no data")
		return &defines.Hover{}, nil
	}

	span := chunk.GetLineColumnSingeCharSpan(line, column)
	hoveredNode, ancestors, ok := chunk.GetNodeAndChainAtSpan(span)

	if !ok || hoveredNode == nil {
		logs.Println("no data")
		return &defines.Hover{}, nil
	}

	//help about manifest sections & lthread meta sections
	help, ok := getSectionHelp(hoveredNode, ancestors)
	if ok {
		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: help,
			},
		}, nil
	}

	//help about tag or attribute
	xmlElementInfo, hasXmlElementInfo := getXmlElementInfo(hoveredNode, ancestors)

	mostSpecificVal, ok := state.SymbolicData.GetMostSpecificNodeValue(hoveredNode)
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
	var stringifiedHoveredNodeValue string

	//try getting the hovered node's value
	{
		utils.PanicIfErr(symbolic.PrettyPrint(mostSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
		var ok bool
		lessSpecificVal, ok = state.SymbolicData.GetLessSpecificNodeValue(hoveredNode)
		if ok {
			w.Write(utils.StringAsBytes("\n\n# less specific\n"))
			utils.PanicIfErr(symbolic.PrettyPrint(lessSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
		}

		w.Flush()
		stringifiedHoveredNodeValue = strings.ReplaceAll(buff.String(), "\n\r", "\n")
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
	if stringifiedHoveredNodeValue != "" {
		codeBlock = "```inox\n" + stringifiedHoveredNodeValue + "\n```"
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

func getSectionHelp(n parse.Node, ancestors []parse.Node) (string, bool) {
	if len(ancestors) < 4 {
		return "", false
	}

	//check the hovered node is the key of an object property
	objProp, ok := ancestors[len(ancestors)-1].(*parse.ObjectProperty)
	if !ok || objProp.Key != n || !utils.Implements[*parse.ObjectLiteral](ancestors[len(ancestors)-2]) {
		return "", false
	}

	object := ancestors[len(ancestors)-2].(*parse.ObjectLiteral)
	propName := objProp.Name()
	grandparent := ancestors[len(ancestors)-3]

	switch gp := grandparent.(type) {
	case *parse.Manifest:
		sectionName := propName
		//hovered node is a manifest section's name
		help, ok := help_ns.HelpFor(fmt.Sprintf("manifest/%s-section", sectionName), help_ns.HelpMessageConfig{
			Format: help_ns.MarkdownFormat,
		})

		if ok {
			return help, true
		}
	case *parse.SpawnExpression:
		sectionName := propName
		if object == gp.Meta {
			//hovered node is a lthread meta section's name
			help, ok := help_ns.HelpFor(fmt.Sprintf("lthreads/%s-section", sectionName), help_ns.HelpMessageConfig{
				Format: help_ns.MarkdownFormat,
			})
			if ok {
				return help, true
			}
		}
	}
	return "", false
}
