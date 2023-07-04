package html_ns

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed data.json
	STANDARD_DATA_JSON []byte
	STANDARD_DATA      StandardData

	VOID_HTML_TAG_NAMES []string
)

func init() {
	utils.PanicIfErr(json.Unmarshal(STANDARD_DATA_JSON, &STANDARD_DATA))

	for _, tag := range STANDARD_DATA.Tags {
		if tag.Void {
			VOID_HTML_TAG_NAMES = append(VOID_HTML_TAG_NAMES, tag.Name)
		}
	}
}

type StandardData struct {
	Version          float64             `json:"version"`
	Tags             []TagData           `json:"tags"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}

type TagData struct {
	Name        string          `json:"name"`
	Description any             `json:"description"` //string | MarkupContent
	Attributes  []AttributeData `json:"attributes"`
	Void        bool            `json:"void"`
	References  []DataReference `json:"references"`
}

type AttributeData struct {
	Name        string               `json:"name"`
	Description any                  `json:"description"` //string | MarkupContent
	ValueSet    string               `json:"valueSet"`
	Values      []AttributeValueData `json:"values"`
	References  []DataReference      `json:"references"`
}

func (d AttributeData) DescriptionText() string {
	description, ok := d.Description.(string)
	if ok {
		return description
	}

	markupContent, ok := d.Description.(map[string]any)
	if ok {
		//TODO: remove markdown formatting
		return markupContent["value"].(string)
	}

	return ""
}

type DataReference struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type AttributeValueSet struct {
	Name   string               `json:"name"`
	Values []AttributeValueData `json:"values"`
}

type AttributeValueData struct {
	Values      string          `json:"name"`
	Description any             `json:"description"` //string | MarkupContent
	References  []DataReference `json:"references"`
}

// GetTagData returns the standard data about a tag (e.g "img", "p"), the returned data should NOT be modified.
func GetTagData(name string) (TagData, bool) {
	name = strings.ToLower(name)

	for _, tag := range STANDARD_DATA.Tags {
		if tag.Name == name {
			return tag, true
		}
	}
	return TagData{}, false
}

// GetTagData returns the specific attributes of a tag (e.g "src" for "img"), the returned data should NOT be modified.
func GetTagSpecificAttributes(name string) ([]AttributeData, bool) {
	data, ok := GetTagData(name)
	if !ok {
		return nil, false
	}
	return data.Attributes, true
}

// GetTagData returns the specific attributes of a tag and global attributes, the returned data should NOT be modified except the slice itself.
func GetAllTagAttributes(name string) ([]AttributeData, bool) {
	tagData, ok := GetTagData(name)
	if !ok {
		return nil, false
	}

	var data []AttributeData

	data = append(data, tagData.Attributes...)
	data = append(data, STANDARD_DATA.GlobalAttributes...)

	return data, true
}
