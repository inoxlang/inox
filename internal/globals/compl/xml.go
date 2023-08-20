package compl

import (
	"encoding/json"
	"strings"

	"github.com/inoxlang/inox/internal/globals/html_ns"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func findXmlAttributeNameCompletions(ident *parse.IdentifierLiteral, parent *parse.XMLAttribute, ancestors []parse.Node) (completions []Completion) {
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
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

func findXMLAttributeValueCompletions(str *parse.QuotedStringLiteral, parent *parse.XMLAttribute, ancestors []parse.Node) (completions []Completion) {
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
	if !ok {
		return
	}

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":
		return findHtmlAttributeValueCompletions(str, parent, tagName, ancestors)
	}

	return
}

func findTagNameAndNamespace(ancestors []parse.Node) (string, *parse.IdentifierLiteral, bool) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return "", nil, false
	}

	openingElem := ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
	tagIdent, ok := openingElem.Name.(*parse.IdentifierLiteral)
	if !ok {
		return "", nil, false
	}

	tagName := tagIdent.Name

	namespace, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return "", nil, false
	}

	return tagName, namespace, true
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

func findHtmlAttributeValueCompletions(str *parse.QuotedStringLiteral, parent *parse.XMLAttribute, tagName string, ancestors []parse.Node) (completions []Completion) {
	attrIdent, ok := parent.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	attrName := attrIdent.Name

	set, ok := html_ns.GetAttributeValueSet(attrName, tagName)
	if !ok {
		return
	}

	for _, attrValueData := range set.Values {
		if !strings.HasPrefix(attrValueData.Name, str.Value) {
			continue
		}

		s := string(utils.Must(json.Marshal(attrValueData.Name)))

		completions = append(completions, Completion{
			ShownString: s,
			Value:       s,
			Kind:        defines.CompletionItemKindProperty,
			Detail:      attrValueData.DescriptionText(),
		})
	}

	return
}
