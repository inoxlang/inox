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
	Keywords             map[string]KeywordInfo      `yaml:"keywords"`
	ByTokenType          map[hscode.TokenType]string `yaml:"token-types"`
	FeatureStartExamples []AttributeStartExample     `yaml:"feature-start-examples"`
	CommandExamples      []AttributeStartExample     `yaml:"command-examples"`
}

type AttributeStartExample struct {
	Code                  string `yaml:"code"`
	ShortExplanation      string `yaml:"short-explanation,omitempty"`
	MarkdownDocumentation string `yaml:"documentation,omitempty"`
}

type CommandExample struct {
	Code                  string `yaml:"code"`
	ShortExplanation      string `yaml:"short-explanation,omitempty"`
	MarkdownDocumentation string `yaml:"documentation,omitempty"`
}

func init() {
	if err := yaml.Unmarshal(utils.StringAsBytes(HELP_DATA_YAML), &HELP_DATA); err != nil {
		log.Panicf("error while parsing hyperscript.yaml: %s", err)
	}

	for name, info := range HELP_DATA.Keywords {
		info.Name = name
		HELP_DATA.Keywords[name] = info
	}
}

// GetKeywordsByPrefix returns the lists of keywords starting with $s (case insensitive).
// If $s is empty all keywords are returned.
func GetKeywordsByPrefix(s string) (keywords []KeywordInfo) {
	s = strings.ToLower(s)

	for token, info := range HELP_DATA.Keywords {
		if strings.HasPrefix(token, s) {
			keywords = append(keywords, info)
		}
	}

	return
}

type KeywordInfo struct {
	Name              string
	DocumentationLink string      `yaml:"documentation"`
	Kind              KeywordKind `yaml:"kind,omitempty"`
}

type KeywordKind string

const (
	UnspecifiedKeywordKind KeywordKind = ""
	FeatureKeyword         KeywordKind = "feature"
	CommandKeyword         KeywordKind = "command"
)
