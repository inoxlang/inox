package html_ns

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

var (
	//go:embed pseudo-htmx-data.json
	PSEUDO_HTMX_DATA_JSON              []byte
	PSEUDO_HTMX_DATA                   PseudoHtmxData
	PSEUDO_HTMX_ATTR_NAMES             []string
	SHORTEST_PSEUDO_HTMX_ATTR_NAME_LEN int = 100
)

type PseudoHtmxData struct {
	Version          float64             `json:"version"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}

func init() {
	utils.PanicIfErr(json.Unmarshal(PSEUDO_HTMX_DATA_JSON, &PSEUDO_HTMX_DATA))
	for _, attr := range PSEUDO_HTMX_DATA.GlobalAttributes {
		PSEUDO_HTMX_ATTR_NAMES = append(PSEUDO_HTMX_ATTR_NAMES, attr.Name)
		if SHORTEST_PSEUDO_HTMX_ATTR_NAME_LEN > len(attr.Name) {
			SHORTEST_PSEUDO_HTMX_ATTR_NAME_LEN = len(attr.Name)
		}
	}
	sort.Strings(PSEUDO_HTMX_ATTR_NAMES)

}

func isPseudoHtmxAttribute(name string) bool {
	if len(name) < SHORTEST_PSEUDO_HTMX_ATTR_NAME_LEN {
		return false
	}
	index := sort.SearchStrings(PSEUDO_HTMX_ATTR_NAMES, name)
	return index < len(PSEUDO_HTMX_ATTR_NAMES) && PSEUDO_HTMX_ATTR_NAMES[index] == name
}

func transpilePseudoHtmxAttribute(attr core.XMLAttribute, currentOutput *[]html.Attribute) error {
	trimmedName := strings.TrimPrefix(attr.Name(), "hx-")

	switch trimmedName {
	case "lazy-load":
		attrValue, ok := attr.Value().(core.StringLike)
		if !ok {
			return fmt.Errorf(`invalid value for attribute %s: a string is expected (e.g. "/users")`, attr.Name())
		}

		*currentOutput = append(*currentOutput,
			html.Attribute{
				Key: "hx-trigger",
				Val: "load",
			},
			html.Attribute{
				Key: "hx-get",
				Val: attrValue.GetOrBuildString(),
			},
		)
	case "post":
		attrValue, ok := attr.Value().(core.StringLike)
		if !ok {
			return fmt.Errorf(`invalid value for attribute %s: a string is expected (e.g. "/users")`, attr.Name())
		}

		*currentOutput = append(*currentOutput,
			html.Attribute{
				Key: "hx-post",
				Val: attrValue.GetOrBuildString(),
			},
			html.Attribute{
				Key: "hx-ext",
				Val: "json-enc",
			},
		)
	}
	return nil
}
