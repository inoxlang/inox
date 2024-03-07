package codecompletion

import (
	"unicode"

	"github.com/inoxlang/inox/internal/globals/globalnames"
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
	switch namespace {
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
	namespace, ok := findXMLNamespaceName(ancestors)
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

	switch namespace {
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
	switch namespace {
	case "html":
		return findHtmlAttributeNameCompletions(ident, parent, tagName, ancestors)
	}

	return
}

// findXMLAttributeValueCompletions finds completions for atribute values inside an Inox XML opening element,
// this is based on the namespace (e.g. html) of the closest Inox XML expression.
func findXMLAttributeValueCompletions(strLiteral parse.SimpleValueLiteral, parent *parse.XMLAttribute, search completionSearch) (completions []Completion) {
	tagName, namespace, ok := findTagNameAndNamespace(search.ancestorChain)
	if !ok {
		return
	}

	//TODO: use symbolic data in order to support aliases
	switch namespace {
	case "html":
		return findHtmlAttributeValueCompletions(strLiteral, parent, tagName, search)
	}

	return
}

// findXMLNamespaceName finds the namespace of the closest Inox XML expression.
func findXMLNamespaceName(ancestors []parse.Node) (string, bool) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return "", false
	}

	if xmlExpr.Namespace == nil {
		return globalnames.HTML_NS, true
	}

	namespaceIdent, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return "", false
	}

	return namespaceIdent.Name, true
}

// findXMLNamespace finds the tag name and namespace of the closest Inox XML expression.
func findTagNameAndNamespace(ancestors []parse.Node) (tag string, ns string, _ bool) {
	xmlExpr, _, found := parse.FindClosest(ancestors, (*parse.XMLExpression)(nil))
	if !found {
		return "", "", false
	}

	openingElem, ok := ancestors[len(ancestors)-1].(*parse.XMLOpeningElement)
	if !ok {
		openingElem = ancestors[len(ancestors)-2].(*parse.XMLOpeningElement)
	}
	tagIdent, ok := openingElem.Name.(*parse.IdentifierLiteral)
	if !ok {
		return "", "", false
	}

	tagName := tagIdent.Name

	if xmlExpr.Namespace == nil {
		ns = globalnames.HTML_NS
	} else if namespaceIdent, ok := xmlExpr.Namespace.(*parse.IdentifierLiteral); ok {
		ns = namespaceIdent.Name
	} else {
		return "", "", false
	}

	return tagName, ns, true
}
