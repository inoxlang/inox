package codecompletion

import (
	parse "github.com/inoxlang/inox/internal/parse"
)

func findXmlTagAndTagNameCompletions(ident *parse.IdentifierLiteral, ancestors []parse.Node) (completions []Completion) {
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
	if !ok {
		return
	}

	var xmlElem *parse.XMLElement
	if len(ancestors) >= 2 {
		xmlElem, _ = ancestors[len(ancestors)-2].(*parse.XMLElement)
	}

	suggestWholeTags := xmlElem != nil && xmlElem.Children == nil && xmlElem.Closing == nil

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":
		completions = getHTMLTagNamesWithPrefix(tagName)

		if suggestWholeTags {
			completions = append(completions, findWholeHTMLTagCompletions(tagName, ancestors)...)
		}
	}

	return
}

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

func findXMLAttributeValueCompletions(str *parse.QuotedStringLiteral, parent *parse.XMLAttribute, ancestors []parse.Node, data InputData) (completions []Completion) {
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
	if !ok {
		return
	}

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":
		return findHtmlAttributeValueCompletions(str, parent, tagName, ancestors, data)
	}

	return
}

func findTagNameAndNamespace(ancestors []parse.Node) (string, *parse.IdentifierLiteral, bool) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return "", nil, false
	}

	openingElem, ok := ancestors[len(ancestors)-1].(*parse.XMLOpeningElement)
	if !ok {
		openingElem = ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
	}
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
