package codecompletion

import (
	"unicode"

	parse "github.com/inoxlang/inox/internal/parse"
)

// findXmlTagAndTagNameCompletions finds tag name and whole tag completions based on the namespace (e.g. html) of the closest Inox XML expression.
func findXmlTagAndTagNameCompletions(ident *parse.IdentifierLiteral, search completionSearch) (completions []Completion) {
	ancestors := search.ancestorChain
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
	if !ok {
		return
	}

	var xmlElem *parse.XMLElement
	if len(ancestors) >= 2 {
		xmlElem, _ = ancestors[len(ancestors)-2].(*parse.XMLElement)
	}

	//we suggest whole tags if only the start of the XML element is present: `<name`.
	suggestWholeTags := xmlElem != nil && xmlElem.Base().Span.End == ident.Span.End

	//TODO: use symbolic data in order to support aliases
	switch namespace.Name {
	case "html":
		completions = getHTMLTagNamesWithPrefix(tagName)

		if suggestWholeTags {
			completions = append(completions, findWholeHTMLTagCompletions(tagName, ancestors, false, search.inputData)...)
		}
	}

	return
}

// findXmlTagAndTagNameCompletions finds completions based on the namespace (e.g. html) of the closest Inox XML opening element.
func findXMLOpeningElementInteriorCompletions(openingElem *parse.XMLOpeningElement, search completionSearch) (completions []Completion) {
	ancestors := search.ancestorChain
	namespace, ok := findXMLNamespace(ancestors)
	if !ok {
		return
	}

	if openingElem.Name == nil {
		return nil
	}

	tagName := openingElem.GetName()

	runes := search.chunk.Runes()
	afterName := runes[openingElem.Name.Base().Span.End:openingElem.Span.End]
	onlySpaceAfterTagName := true

	for _, r := range afterName {
		if onlySpaceAfterTagName && !unicode.IsSpace(r) {
			onlySpaceAfterTagName = false
		}
	}

	suggestWholeTags := onlySpaceAfterTagName

	switch namespace.Name {
	case "html":
		if suggestWholeTags {
			completions = append(completions, findWholeHTMLTagCompletions(tagName, ancestors, true, search.inputData)...)
		}
	}

	return
}

// findXmlAttributeNameCompletions finds completions for atribute names inside an Inox XML opening element,
// this is based on the namespace (e.g. html) of the closest Inox XML expression.
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

// findXMLAttributeValueCompletions finds completions for atribute values inside an Inox XML opening element,
// this is based on the namespace (e.g. html) of the closest Inox XML expression.
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

// findXMLNamespace finds the namespace of the closest Inox XML expression.
func findXMLNamespace(ancestors []parse.Node) (*parse.IdentifierLiteral, bool) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return nil, false
	}

	namespace, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return nil, false
	}

	return namespace, true
}

// findXMLNamespace finds the tag name and namespace of the closest Inox XML expression.
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
