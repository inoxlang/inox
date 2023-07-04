package html_ns

import (
	_ "embed"
	"encoding/json"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed data.json
	STANDARD_DATA_JSON []byte
	STANDARD_DATA      StandardData
)

func init() {
	utils.PanicIfErr(json.Unmarshal(STANDARD_DATA_JSON, &STANDARD_DATA))
}

type StandardData struct {
	Version float64   `json:"version"`
	Tags    []TagData `json:"tags"`
}

type TagData struct {
	Name        string          `json:"name"`
	Description any             `json:"description"` //string | markup
	Attributes  []AttributeData `json:"attributes"`
	Void        bool            `json:"void"`
	References  []DataReference `json:"references"`
}

type AttributeData struct {
	Name        string `json:"name"`
	Description any    `json:"description"` //string | markup
	ValueSet    string `json:"valueSet"`
}

type DataReference struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type GlobalAttributeData struct {
	Name        string          `json:"name"`
	Description any             `json:"description"` //string | markup
	References  []DataReference `json:"references"`
}

type AttributeValueSet struct {
	Name   string               `json:"name"`
	Values []AttributeValueData `json:"values"`
}

type AttributeValueData struct {
	Values      string          `json:"name"`
	Description any             `json:"description"` //string | markup
	References  []DataReference `json:"references"`
}
