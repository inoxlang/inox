package hshelp

import (
	_ "embed"
	"log"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
)

//go:embed hyperscript.yaml
var HELP_DATA_YAML string

var HELP_DATA struct {
	Keywords    map[string]string           `yaml:"keywords"`
	ByTokenType map[hscode.TokenType]string `yaml:"token-types"`
}

func init() {
	if err := yaml.Unmarshal(utils.StringAsBytes(HELP_DATA_YAML), &HELP_DATA); err != nil {
		log.Panicf("error while parsing hyperscript.yaml: %s", err)
	}
}

// GetKeywordsByPrefix returns the lists of keywords starting with $s (case insensitive).
// If $s is empty all keywords are returned.
func GetKeywordsByPrefix(s string) (keywords []KeywordInfo) {
	s = strings.ToLower(s)

	for token, docLink := range HELP_DATA.Keywords {
		if strings.HasPrefix(token, s) {
			keywords = append(keywords, KeywordInfo{
				Name:              token,
				DocumentationLink: docLink,
			})
		}
	}

	return
}

type KeywordInfo struct {
	Name              string
	DocumentationLink string
}
