package compl

import (
	"strings"

	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
	parse "github.com/inoxlang/inox/internal/parse"
)

func findXmlAttributeNameCompletions(ident *parse.IdentifierLiteral, parent *parse.XMLAttribute, ancestors []parse.Node) (completions []Completion) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return
	}

	openingElem := ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
	tagIdent, ok := openingElem.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	tagName := tagIdent.Name

	namespace, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":
		return findHtmlAttributeNameCompletions(ident, parent, tagName, ancestors)
	}

	return
}

func findHtmlAttributeNameCompletions(ident *parse.IdentifierLiteral, parent *parse.XMLAttribute, tagName string, ancestors []parse.Node) (completions []Completion) {
	attributes, ok := html_ns.GetAllTagAttributes(tagName)
	if !ok {
		return
	}

	attrName := ident.Name

	for _, attr := range attributes {
		if !strings.HasPrefix(attr.Name, attrName) {
			continue
		}

		completions = append(completions, Completion{
			ShownString: attr.Name,
			Value:       attr.Name,
			Kind:        defines.CompletionItemKindProperty,
			Detail:      attr.DescriptionText(),
		})
	}

	return
}
