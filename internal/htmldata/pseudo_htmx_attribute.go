package htmldata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	utils "github.com/inoxlang/inox/internal/utils/common"
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

func IsPseudoHtmxAttribute(name string) bool {
	if len(name) < SHORTEST_PSEUDO_HTMX_ATTR_NAME_LEN {
		return false
	}
	index := sort.SearchStrings(PSEUDO_HTMX_ATTR_NAMES, name)
	return index < len(PSEUDO_HTMX_ATTR_NAMES) && PSEUDO_HTMX_ATTR_NAMES[index] == name
}

func GetEquivalentAttributesNamesToPseudoHTMXAttribute(name string) (names [3]string, count int) {
	trimmedName := strings.TrimPrefix(name, "hx-")

	switch trimmedName {
	case "lazy-load":
		count = 2
		names[0] = "hx-trigger"
		names[1] = "hx-get"
	case "post-json", "patch-json", "put-json":
		method, _, _ := strings.Cut(trimmedName, "-")

		count = 2
		names[0] = "hx-" + method
		names[1] = "hx-ext"
	}

	return
}

func GetEquivalentsToPseudoHtmxAttribute(name string, value core.Value) (attributes [3]html.Attribute, count int, err error) {
	//Most updates to this function should be followed by updates to GetEquivalentAttributesNamesToPseudoHTMXAttribute.

	trimmedName := strings.TrimPrefix(name, "hx-")

	switch trimmedName {
	case "lazy-load":
		attrValue, ok := value.(core.StringLike)
		if !ok {
			err = fmt.Errorf(`invalid value for attribute %s: a string is expected (e.g. "/users")`, name)
			return
		}

		count = 2
		attributes[0] = html.Attribute{
			Key: "hx-trigger",
			Val: "load",
		}
		attributes[1] = html.Attribute{
			Key: "hx-get",
			Val: attrValue.GetOrBuildString(),
		}
	case "post-json", "patch-json", "put-json":
		attrValue, ok := value.(core.StringLike)
		if !ok {
			err = fmt.Errorf(`invalid value for attribute %s: a string is expected (e.g. "/users")`, name)
			return
		}

		method, _, _ := strings.Cut(trimmedName, "-")
		count = 2
		attributes[0] = html.Attribute{
			Key: "hx-" + method,
			Val: attrValue.GetOrBuildString(),
		}
		attributes[1] = html.Attribute{
			Key: "hx-ext",
			Val: "json-form",
		}
	}
	return
}
