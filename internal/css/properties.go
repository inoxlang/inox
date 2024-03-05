package css

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
)

var (
	//go:embed properties-w3c.json
	RAW_PROPERTY_LIST []byte
	PROPERTY_LIST     struct {
		Properties []string `json:"properties"`
	}
)

func init() {
	err := json.Unmarshal(RAW_PROPERTY_LIST, &PROPERTY_LIST)
	if err != nil {
		panic(err)
	}

	sort.Strings(PROPERTY_LIST.Properties)
}

func ForEachPropertyName(prefix string, fn func(name string) error) error {
	properties := PROPERTY_LIST.Properties

	if prefix == "" {
		for _, name := range properties {
			err := fn(name)
			if err != nil {
				return err
			}
		}
	} else {
		firstChar := prefix[0]
		startIndex := sort.SearchStrings(properties, string(firstChar))
		if startIndex >= len(properties) {
			return nil
		}

		for i := startIndex; i < len(properties) && PROPERTY_LIST.Properties[i][0] == firstChar; i++ {
			name := properties[i]
			if strings.HasPrefix(name, prefix) {
				err := fn(name)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
