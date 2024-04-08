package codecompletion

import (
	"unicode"

	"github.com/inoxlang/inox/internal/globals/globalnames"
	parse "github.com/inoxlang/inox/internal/parse"
)

// findTagAndTagNameCompletions finds tag name and whole tag completions based on the namespace (e.g. html) of the closest Inox markup expression.
func findTagAndTagNameCompletions(ident *parse.IdentifierLiteral, search completionSearch) (completions []Completion) {
	ancestors := search.ancestorChain
	tagName, namespace, ok := findTagNameAndNamespace(ancestors)
	if !ok {
		return
	}

	var markupElem *parse.MarkupElement
	if len(ancestors) >= 2 {
		markupElem, _ = ancestors[len(ancestors)-2].(*parse.MarkupElement)
	}

	//we suggest whole tags if only the start of the markup element is present: `<name`.
	suggestWholeTags := markupElem != nil && markupElem.Base().Span.End == ident.Span.End

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

// findOpeningTagInteriorCompletions finds completions based on the namespace (e.g. html) of the closest Inox markup opening element.
func findOpeningTagInteriorCompletions(openingElem *parse.MarkupOpeningTag, search completionSearch) (completions []Completion) {
	ancestors := search.ancestorChain
	namespace, ok := findMarkupNamespaceName(ancestors)
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

// findMarkupAttributeNameCompletions finds completions for atribute names inside an opening tag,
// this is based on the namespace (e.g. html) of the closest Inox markup expression.
func findMarkupAttributeNameCompletions(ident *parse.IdentifierLiteral, parent *parse.MarkupAttribute, ancestors []parse.Node) (completions []Completion) {
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

// findMarkupAttributeValueCompletions finds completions for atribute values inside an Inox markup opening element,
// this is based on the namespace (e.g. html) of the closest Inox markup expression.
func findMarkupAttributeValueCompletions(strLiteral parse.SimpleValueLiteral, parent *parse.MarkupAttribute, search completionSearch) (completions []Completion) {
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

// findMarkupNamespaceName finds the namespace of the closest Inox markup expression.
func findMarkupNamespaceName(ancestors []parse.Node) (string, bool) {
	markupExpr, _, found := parse.FindClosest(ancestors, (*parse.MarkupExpression)(nil))
	if !found {
		return "", false
	}

	if markupExpr.Namespace == nil {
		return globalnames.HTML_NS, true
	}

	namespaceIdent, ok := markupExpr.Namespace.(*parse.IdentifierLiteral)
	if !ok {
		return "", false
	}

	return namespaceIdent.Name, true
}

// findmarkupNamespace finds the tag name and namespace of the closest Inox markup expression.
func findTagNameAndNamespace(ancestors []parse.Node) (tag string, ns string, _ bool) {
	markupExpr, _, found := parse.FindClosest(ancestors, (*parse.MarkupExpression)(nil))
	if !found {
		return "", "", false
	}

	openingElem, ok := ancestors[len(ancestors)-1].(*parse.MarkupOpeningTag)
	if !ok {
		openingElem = ancestors[len(ancestors)-2].(*parse.MarkupOpeningTag)
	}
	tagIdent, ok := openingElem.Name.(*parse.IdentifierLiteral)
	if !ok {
		return "", "", false
	}

	tagName := tagIdent.Name

	if markupExpr.Namespace == nil {
		ns = globalnames.HTML_NS
	} else if namespaceIdent, ok := markupExpr.Namespace.(*parse.IdentifierLiteral); ok {
		ns = namespaceIdent.Name
	} else {
		return "", "", false
	}

	return tagName, ns, true
}
