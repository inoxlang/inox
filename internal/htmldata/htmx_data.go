package htmldata

import (
	_ "embed"
	"encoding/json"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed htmx-data.json
	HTMX_DATA_JSON []byte
	HTMX_DATA      HtmxData
)

func init() {
	utils.PanicIfErr(json.Unmarshal(HTMX_DATA_JSON, &HTMX_DATA))

	for _, tag := range STANDARD_DATA.Tags {
		if tag.Void {
			VOID_HTML_TAG_NAMES = append(VOID_HTML_TAG_NAMES, tag.Name)
		}
	}
}

type HtmxData struct {
	Version          float64             `json:"version"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}
