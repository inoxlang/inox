package codecompletion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/mimeconsts"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

// This file contains completion logic for HTML and HTMX.

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

	if inputData.ServerAPI != nil && strings.HasPrefix(attrValue, "/") && (strings.HasPrefix(attrName, "hx-") || attrName == "href") {
		//local server

		api := inputData.ServerAPI

		var endpointPaths []string
		api.ForEachHandlerModule(func(mod *core.Module, endpoint *spec.ApiEndpoint, operation spec.ApiOperation) error {
			addEndpoint := false

			switch attrName {
			case "href":
				if tagName == "a" {
					addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "GET"
				}
			case "hx-get":
				addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "GET"
			case "hx-post-json":
				addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "POST"
			case "hx-patch-json":
				addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "PATCH"
			case "hx-put-json":
				addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "PUT"
			case "hx-delete":
				addEndpoint = endpoint.CatchAll() || operation.HttpMethod() == "DELETE"
			}

			if addEndpoint {
				endpointPaths = append(endpointPaths, endpoint.PathWithParams())
			}
			return nil
		})

		for _, path := range endpointPaths {
			completions = append(completions, Completion{
				ShownString: path,
				Value:       `"` + path + `"`,
				Kind:        defines.CompletionItemKindProperty,
			})
		}
	}

	return
}

func findWholeHTMLTagCompletions(tagName string, ancestors []parse.Node, includeAngleBracket bool, inputData InputData) (completions []Completion) {
	switch tagName {
	case "fo", "for", "form":
		if inputData.ServerAPI == nil {
			return
		}
		api := inputData.ServerAPI

		prefix := ""
		if includeAngleBracket {
			prefix = "<"
		}

		api.ForEachHandlerModule(func(mod *core.Module, endpoint *spec.ApiEndpoint, operation spec.ApiOperation) error {
			//ignore non-mutating methods.
			if !endpoint.CatchAll() && !http_ns.IsMutationMethod(operation.HttpMethod()) {
				return nil
			}

			method := "post"
			hxAttribute := "hx-post-json"
			path := endpoint.PathWithParams()

			inputsBuf := bytes.NewBuffer(nil)

			if !endpoint.CatchAll() { //if operation is defined
				method = operation.HttpMethod()
				switch method {
				case "DELETE":
					hxAttribute = "hx-delete"
				case "PATCH":
					hxAttribute = "hx-patch-json"
				case "PUT":
					hxAttribute = "hx-put-json"
				}

				bodyPattern, ok := operation.JSONRequestBodyPattern()
				if ok {
					writeHtmlInputs(inputsBuf, formInputGeneration{
						required: true,
						pattern:  bodyPattern,
					})
				}
			}

			completions = append(completions, Completion{
				ShownString: prefix + "form " + method + " " + path,
				Value: prefix +
					"form " + hxAttribute + `="` + path + `">` +
					inputsBuf.String() +
					"\n\t<button type=\"submit\">Submit</button>" +
					"\n</form>",
				Kind: defines.CompletionItemKindProperty,
			})
			return nil
		})
		return
	}
	return
}

func writeHtmlInputs(w *bytes.Buffer, gen formInputGeneration) {
	switch p := gen.pattern.(type) {
	case *core.ObjectPattern:
		p.ForEachEntry(func(entry core.ObjectPatternEntry) error {
			name := entry.Name
			if gen.parent != "" {
				name = gen.parent + "." + name
			}

			required := gen.required && !entry.IsOptional

			if isTerminalFormParamPattern(entry.Pattern) {
				writeTerminalHtmlInputs(w, formInputGeneration{
					terminalInputsName: name,
					required:           required,
					pattern:            entry.Pattern,
				})
			} else {
				writeHtmlInputs(w, formInputGeneration{
					parent:   name,
					required: required,
					pattern:  entry.Pattern,
				})
			}
			return nil
		})
	case *core.RecordPattern:
		p.ForEachEntry(func(entry core.RecordPatternEntry) error {
			name := entry.Name
			if gen.parent != "" {
				name = gen.parent + "." + name
			}

			required := gen.required && !entry.IsOptional

			if isTerminalFormParamPattern(entry.Pattern) {
				writeTerminalHtmlInputs(w, formInputGeneration{
					terminalInputsName: name,
					required:           required,
					pattern:            entry.Pattern,
				})
			} else {
				writeHtmlInputs(w, formInputGeneration{
					parent:   name,
					required: required,
					pattern:  entry.Pattern,
				})
			}
			return nil
		})
	case *core.ListPattern:
		exactElemCount, ok := p.ExactElementCount()
		if ok {
			for i := 0; i < exactElemCount; i++ {
				name := gen.parent + "[" + strconv.Itoa(i) + "]"
				elementPattern := utils.MustGet(p.ElementPatternAt(i))

				if isTerminalFormParamPattern(elementPattern) {
					writeTerminalHtmlInputs(w, formInputGeneration{
						terminalInputsName: name,
						required:           gen.required,
						pattern:            elementPattern,
					})
				} else {
					writeHtmlInputs(w, formInputGeneration{
						parent:   name,
						required: gen.required,
						pattern:  elementPattern,
					})
				}
			}
		} else {
			minCount := p.MinElementCount()
			maxCount := p.MaxElementCount()
			if minCount != 0 || maxCount == core.DEFAULT_LIST_PATTERN_MAX_ELEM_COUNT {
				w.WriteString("<!-- failed to generate inputs for elements of ")
				w.WriteString(gen.parent)
				w.WriteString(" -->")
				return
			}

			w.WriteString("<!-- failed to generate inputs for elements of ")
			w.WriteString(gen.parent)
			w.WriteString(" -->")
		}
	case *core.TuplePattern:
		exactElemCount, ok := p.ExactElementCount()
		if ok {
			for i := 0; i < exactElemCount; i++ {
				name := gen.parent + "[" + strconv.Itoa(i) + "]"
				elementPattern := utils.MustGet(p.ElementPatternAt(i))

				if isTerminalFormParamPattern(elementPattern) {
					writeTerminalHtmlInputs(w, formInputGeneration{
						terminalInputsName: name,
						required:           gen.required,
						pattern:            elementPattern,
					})
				} else {
					writeHtmlInputs(w, formInputGeneration{
						parent:   name,
						required: gen.required,
						pattern:  elementPattern,
					})
				}
			}
		} else {
			w.WriteString("<!-- failed to generate inputs for elements of ")
			w.WriteString(gen.parent)
			w.WriteString(" -->")
		}
	default:
		if isTerminalFormParamPattern(p) {
			writeTerminalHtmlInputs(w, formInputGeneration{
				parent:   gen.parent,
				required: gen.required,
				pattern:  p,
			})
		}
	}
}

func isTerminalFormParamPattern(p core.Pattern) bool {
	switch p.(type) {
	case *core.IntRangePattern, *core.FloatRangePattern:
		return true
	}

	switch p {
	case core.INT_PATTERN, core.FLOAT_PATTERN, core.BOOL_PATTERN,
		core.YEAR_PATTERN, core.DATE_PATTERN, core.DATETIME_PATTERN, core.DURATION_PATTERN,
		core.EMAIL_ADDR_PATTERN, core.STRING_PATTERN, core.STR_PATTERN,
		core.URL_PATTERN:
		return true
	}
	return false
}

type formInputGeneration struct {
	terminalInputsName string
	required           bool
	pattern            core.Pattern
	parent             string
}

func writeTerminalHtmlInputs(w *bytes.Buffer, gen formInputGeneration) (supported bool) {
	type input struct {
		//type attribute
		typ string

		//value attribute
		value string

		//pattern attribute, it can only be defined for the following types: text, search, url, tel, email, password.
		pattern string

		//additional attributes
		//https://developer.mozilla.org/en-US/docs/Web/HTML/Element/Input#attributes
		additional [][2]string

		//comment added after the <input> element.
		comment string
	}
	var inputs []input

	switch p := gen.pattern.(type) {
	case *core.IntRangePattern:
		input := input{}
		intRange := p.Range()

		if intRange.HasKnownStart() {
			min := fmt.Sprintf("%d", intRange.KnownStart())
			input.additional = append(input.additional, [2]string{"min", min})
		}

		end := intRange.InclusiveEnd()
		if end < math.MaxInt64 {
			max := fmt.Sprintf("%d", end)
			input.additional = append(input.additional, [2]string{"max", max})
		}

		inputs = append(inputs, input)
	case *core.FloatRangePattern:
		input := input{
			typ: "number",
		}

		floatRange := p.Range()
		if floatRange.HasKnownStart() {
			min := fmt.Sprintf("%f", floatRange.KnownStart())
			input.additional = append(input.additional, [2]string{"min", min})
		}

		end := floatRange.InclusiveEnd()
		if end < math.MaxFloat64 {
			max := fmt.Sprintf("%f", end)
			input.additional = append(input.additional, [2]string{"max", max})
		}

		inputs = append(inputs, input)
	default:
		switch p {
		case core.INT_PATTERN:
			inputs = append(inputs, input{
				typ:        "number",
				additional: [][2]string{{"step", "1"}},
			})
		case core.FLOAT_PATTERN:
			inputs = append(inputs, input{typ: "number"})
		case core.BOOL_PATTERN:
			inputs = append(inputs, input{typ: "checkbox", value: "yes"})
		case core.EMAIL_ADDR_PATTERN:
			inputs = append(inputs, input{typ: "email"})
		case core.YEAR_PATTERN:
			inputs = append(inputs, input{
				typ:        "number",
				additional: [][2]string{{"step", "1"}},
			})
		case core.DATE_PATTERN:
			inputs = append(inputs, input{typ: "date"})
		case core.DATETIME_PATTERN:
			inputs = append(inputs, input{typ: "datetime-local"})
		case core.DURATION_PATTERN:
			inputs = append(inputs, input{typ: "number"})
		case core.STRING_PATTERN, core.STR_PATTERN:
			inputs = append(inputs, input{typ: "text"})
		case core.URL_PATTERN:
			inputs = append(inputs, input{typ: "url"})
		}
	}

	//write the inputs

	for _, input := range inputs {
		w.WriteString("\n\t<input name=\"")
		w.WriteString(gen.terminalInputsName) //TODO: encode
		w.WriteByte('"')

		w.WriteString(" placeholder=\"")
		w.WriteString(gen.terminalInputsName) //TODO: encode
		w.WriteByte('"')

		if input.typ != "" {
			w.WriteString(" type=\"")
			w.WriteString(input.typ)
			w.WriteByte('"')
		}

		if input.pattern != "" {
			w.WriteString(" pattern=\"")
			w.WriteString(input.pattern) //TODO: encode
			w.WriteByte('"')
		}

		if input.value != "" {
			w.WriteString(" value=\"")
			w.WriteString(input.value) //TODO: encode
			w.WriteByte('"')
		}

		for _, additionalAttribute := range input.additional {
			w.WriteByte(' ')
			w.WriteString(additionalAttribute[0])
			w.WriteString(`="`)
			w.WriteString(additionalAttribute[1]) //TODO: encode
			w.WriteByte('"')
		}

		if gen.required {
			w.WriteString(` required`)
		}

		//Close the input with '/>' because Inox's JSX-like syntax
		//is not aware of void tags.
		w.WriteString(`/>`)

		if input.comment != "" {
			w.WriteString(" <!-- ")
			w.WriteString(input.comment)
			w.WriteString(" -->")
		}

	}

	return true
}
