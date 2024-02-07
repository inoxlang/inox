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

func (d TagData) DescriptionText() string {
	return getDescriptionText(d.Description)
}

func (d TagData) DescriptionContent() string {
	return getDescriptionContent(d.Description)
}

type AttributeData struct {
	Name        string          `json:"name"`
	Description any             `json:"description"` //string | MarkupContent
	ValueSet    string          `json:"valueSet"`
	References  []DataReference `json:"references"`
}

func (d AttributeData) DescriptionText() string {
	return getDescriptionText(d.Description)
}

func (d AttributeData) DescriptionContent() string {
	return getDescriptionContent(d.Description)
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
	Name        string          `json:"name"`
	Description any             `json:"description"` //string | MarkupContent
	References  []DataReference `json:"references"`
}

func (d AttributeValueData) DescriptionText() string {
	return getDescriptionText(d.Description)
}

func (d AttributeValueData) DescriptionContent() string {
	return getDescriptionContent(d.Description)
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

func GetAllTagAttributes(name string) ([]AttributeData, bool) {
	tagData, ok := GetTagData(name)
	if !ok {
		return nil, false
	}

	var data []AttributeData

	data = append(data, tagData.Attributes...)
	data = append(data, STANDARD_DATA.GlobalAttributes...)
	data = append(data, HTMX_DATA.GlobalAttributes...)
	data = append(data, PSEUDO_HTMX_DATA.GlobalAttributes...)
	data = append(data, JSONFORM_DATA.GlobalAttributes...)

	return data, true
}

func GetAttributeValueSet(name string, tagName string) (set AttributeValueSet, found bool) {
	attributes, ok := GetTagSpecificAttributes(tagName)
	if !ok {
		return
	}

	for _, attr := range attributes {
		if attr.Name == name {
			set, found = getValueSet(attr.ValueSet)
			return
		}
	}

	return
}

func getValueSet(name string) (AttributeValueSet, bool) {
	for _, set := range STANDARD_DATA.ValueSets {
		if set.Name == name {
			return set, true
		}
	}
	return AttributeValueSet{}, false
}

func getDescriptionText(d any) string {
	description, ok := d.(string)
	if ok {
		return description
	}

	markupContent, ok := d.(map[string]any)
	if ok {
		//TODO: remove markdown formatting
		return markupContent["value"].(string)
	}

	return ""
}

func getDescriptionContent(d any) string {
	description, ok := d.(string)
	if ok {
		return description
	}

	markupContent, ok := d.(map[string]any)
	if ok {
		return markupContent["value"].(string)
	}

	return ""
}
