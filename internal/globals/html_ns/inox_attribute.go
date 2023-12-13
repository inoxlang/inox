package html_ns

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

const (
	INOX_ATTR_PREFIX = "ix-"
)

var (
	//go:embed ix-data.json
	IX_DATA_JSON []byte
	IX_DATA      Ixdata
)

type Ixdata struct {
	Version          float64             `json:"version"`
	GlobalAttributes []AttributeData     `json:"globalAttributes"`
	ValueSets        []AttributeValueSet `json:"valueSets"`
}

func init() {
	utils.PanicIfErr(json.Unmarshal(IX_DATA_JSON, &IX_DATA))
}

func transpileInoxAttribute(attr core.XMLAttribute, currentOutput *[]html.Attribute) error {
	trimmedName := strings.TrimPrefix(attr.Name(), INOX_ATTR_PREFIX)

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
