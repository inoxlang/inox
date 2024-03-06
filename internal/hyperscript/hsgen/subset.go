package hsgen

import (
	_ "embed"
	"regexp"
	"slices"
	"strings"
)

var (
	//go:embed hyperscript.0.9.12.js
	HYPERSCRIPT_0_9_12_JS string

	ADD_COMMAND_LEN                = len("parser.addCommand")
	ADD_FEATURE_LEN                = len("parser.addFeature")
	FEATURE_OR_CMD_DEF_START_REGEX = regexp.MustCompile(`parser\.(addFeature|addCommand)\(`)

	DEFINITIONS   = []Definition{}
	COMMAND_NAMES []string
	FEATURE_NAMES []string
)

type Config struct {
	Commands     []string
	FeatureNames []string
}

func init() {
	hyperscript := HYPERSCRIPT_0_9_12_JS

	//Find all feature and command definition regions.
	for _, pos := range FEATURE_OR_CMD_DEF_START_REGEX.FindAllStringIndex(hyperscript, -1) {
		start := pos[0]
		defStartSubstring := hyperscript[start : start+max(ADD_COMMAND_LEN, ADD_FEATURE_LEN)]
		isFeature := strings.Contains(defStartSubstring, "addFeature")
		kind := FeatureDefinition
		if !isFeature {
			kind = CommandDefinition
		}

		def := GetDefinition(start, kind, hyperscript)
		DEFINITIONS = append(DEFINITIONS, def)

		if isFeature {
			FEATURE_NAMES = append(FEATURE_NAMES, def.Name)
		} else {
			COMMAND_NAMES = append(COMMAND_NAMES, def.Name)
		}
	}
}

// Generate generates a subset of hyperscript.js that only contains the command and feature definitions listed in the configuration.
func Generate(config Config) (string, error) {

	base := HYPERSCRIPT_0_9_12_JS
	prevDefEnd := 0

	builder := strings.Builder{}

	for _, def := range DEFINITIONS {
		beforeDefinition := base[prevDefEnd:def.Start]
		builder.WriteString(beforeDefinition)

		prevDefEnd = def.End

		if def.Kind == FeatureDefinition && slices.Contains(config.FeatureNames, def.Name) {
			//include the definition.
			builder.WriteString(def.Code)
		}

		if def.Kind == CommandDefinition && slices.Contains(config.Commands, def.Name) {
			//include the definition.
			builder.WriteString(def.Code)
		}
	}

	builder.WriteString(base[prevDefEnd:])

	return builder.String(), nil
}
