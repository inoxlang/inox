package html_ns

import (
	_ "embed"
	"encoding/json"
	"sort"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed jsonform-data.json
	PSEUDO_JSONFORM_DATA_JSON       []byte
	JSONFORM_DATA                   PseudoHtmxData
	JSONFORM_ATTR_NAMES             []string
	SHORTEST_JSONFORM_ATTR_NAME_LEN int = 100
)

type JSONFormData struct {
	Version          float64             `json:"version"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}

func init() {
	utils.PanicIfErr(json.Unmarshal(PSEUDO_JSONFORM_DATA_JSON, &JSONFORM_DATA))
	for _, attr := range JSONFORM_DATA.GlobalAttributes {
		JSONFORM_ATTR_NAMES = append(JSONFORM_ATTR_NAMES, attr.Name)
		if SHORTEST_JSONFORM_ATTR_NAME_LEN > len(attr.Name) {
			SHORTEST_JSONFORM_ATTR_NAME_LEN = len(attr.Name)
		}
	}
	sort.Strings(JSONFORM_ATTR_NAMES)

}

func isJsonformAttribute(name string) bool {
	if len(name) < SHORTEST_JSONFORM_ATTR_NAME_LEN {
		return false
	}
	index := sort.SearchStrings(JSONFORM_ATTR_NAMES, name)
	return index < len(JSONFORM_ATTR_NAMES) && JSONFORM_ATTR_NAMES[index] == name
}
