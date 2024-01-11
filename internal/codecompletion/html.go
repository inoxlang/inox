package codecompletion

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/mimeconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func getHTMLTagNamesWithPrefix(prefix string) (completions []Completion) {
	for _, tag := range html_ns.STANDARD_DATA.Tags {
		if strings.HasPrefix(tag.Name, prefix) {
			completions = append(completions, Completion{
				ShownString:           tag.Name,
				Value:                 tag.Name,
				Kind:                  defines.CompletionItemKindProperty,
				LabelDetail:           tag.DescriptionText(),
				MarkdownDocumentation: tag.DescriptionContent(),
			})
		}
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
			ShownString:           attr.Name,
			Value:                 attr.Name,
			Kind:                  defines.CompletionItemKindProperty,
			LabelDetail:           attr.DescriptionText(),
			MarkdownDocumentation: attr.DescriptionContent(),
		})
	}

	return
}

func findHtmlAttributeValueCompletions(
	str *parse.QuotedStringLiteral,
	parent *parse.XMLAttribute,
	tagName string,
	ancestors []parse.Node,
	inputData InputData,
) (completions []Completion) {
	attrIdent, ok := parent.Name.(*parse.IdentifierLiteral)
	if !ok {
		return
	}

	attrName := attrIdent.Name
	attrValue := str.Value

	set, ok := html_ns.GetAttributeValueSet(attrName, tagName)
	if ok {
		for _, attrValueData := range set.Values {
			if !strings.HasPrefix(attrValueData.Name, str.Value) {
				continue
			}

			s := string(utils.Must(json.Marshal(attrValueData.Name)))

			completions = append(completions, Completion{
				ShownString: s,
				Value:       s,
				Kind:        defines.CompletionItemKindProperty,
				LabelDetail: attrValueData.DescriptionText(),
			})
		}
		return
	}

	switch tagName {
	case "link":
		if attrName != "href" {
			break
		}
		//TODO: only add completions if rel=stylesheet

		for _, path := range inputData.StaticFileURLPaths {
			if !strings.HasSuffix(path, ".css") || !strings.HasPrefix(path, attrValue) {
				continue
			}
			completions = append(completions, Completion{
				ShownString: path,
				Value:       `"` + path + `"`,
				Kind:        defines.CompletionItemKindProperty,
			})
		}
	case "script":
		if attrName != "src" {
			break
		}
		for _, path := range inputData.StaticFileURLPaths {
			if !strings.HasSuffix(path, ".js") || !strings.HasPrefix(path, attrValue) {
				continue
			}
			completions = append(completions, Completion{
				ShownString: path,
				Value:       `"` + path + `"`,
				Kind:        defines.CompletionItemKindProperty,
			})
		}
	case "img":
		if attrName != "src" {
			break
		}
		for _, path := range inputData.StaticFileURLPaths {
			if !strings.HasPrefix(path, attrValue) {
				continue
			}

			ext := filepath.Ext(path)
			mimetype := mimeconsts.TypeByExtensionWithoutParams(ext)

			if !slices.Contains(mimeconsts.COMMON_IMAGE_CTYPES, mimetype) {
				continue
			}

			completions = append(completions, Completion{
				ShownString: path,
				Value:       `"` + path + `"`,
				Kind:        defines.CompletionItemKindProperty,
			})
		}
	}

	return
}

func findWholeHTMLTagCompletions(tagName string, ancestors []parse.Node) []Completion {
	switch tagName {
	case "form":
		return nil
	}
	return nil
}

func getForm(prefix string) (completions []Completion) {
	return nil
}
