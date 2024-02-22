package html_ns

import (
	_ "embed"
	"encoding/json"
	"sort"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed htmx-extensions-data.json
	HTMX_EXTENSIONS_DATA_JSON              []byte
	HTMX_EXTENSIONS_DATA                   PseudoHtmxData
	HTMX_EXTENSIONS_ATTR_NAMES             []string
	SHORTEST_HTMX_EXTENSIONS_ATTR_NAME_LEN int = 100
)

type HTMXExtensionsdata struct {
	Version          float64             `json:"version"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}

func init() {
	utils.PanicIfErr(json.Unmarshal(HTMX_EXTENSIONS_DATA_JSON, &HTMX_EXTENSIONS_DATA))
	for _, attr := range HTMX_EXTENSIONS_DATA.GlobalAttributes {
		HTMX_EXTENSIONS_ATTR_NAMES = append(HTMX_EXTENSIONS_ATTR_NAMES, attr.Name)
		if SHORTEST_HTMX_EXTENSIONS_ATTR_NAME_LEN > len(attr.Name) {
			SHORTEST_HTMX_EXTENSIONS_ATTR_NAME_LEN = len(attr.Name)
		}
	}
	sort.Strings(HTMX_EXTENSIONS_ATTR_NAMES)

}
