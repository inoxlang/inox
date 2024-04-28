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

	BUILTIN_DEFINITIONS   = []Definition{}
	COMMAND_NAMES         []string
	BUILTIN_COMMAND_NAMES []string

	FEATURE_NAMES         []string
	BUILTIN_FEATURE_NAMES []string
)

type Config struct {
	RequiredCommands     []string
	RequiredFeatureNames []string
	RequiredDefinitions  []Definition
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
		BUILTIN_DEFINITIONS = append(BUILTIN_DEFINITIONS, def)

		if isFeature {
			BUILTIN_FEATURE_NAMES = append(BUILTIN_FEATURE_NAMES, def.Name)
		} else {
			BUILTIN_COMMAND_NAMES = append(BUILTIN_COMMAND_NAMES, def.Name)
		}
	}
	COMMAND_NAMES = append(COMMAND_NAMES, BUILTIN_COMMAND_NAMES...)
	FEATURE_NAMES = append(FEATURE_NAMES, BUILTIN_FEATURE_NAMES...)
}

// Generate generates a subset of hyperscript.gen.js that only contains the command and feature definitions listed in the configuration.
func Generate(config Config) (string, error) {

	base := HYPERSCRIPT_0_9_12_JS
	prevDefEnd := 0

	builder := strings.Builder{}

	for _, def := range BUILTIN_DEFINITIONS {
		beforeDefinition := base[prevDefEnd:def.Start]
		builder.WriteString(beforeDefinition)

		prevDefEnd = def.End

		if def.Kind == FeatureDefinition && slices.Contains(config.RequiredFeatureNames, def.Name) ||
			def.Kind == CommandDefinition && slices.Contains(config.RequiredCommands, def.Name) ||
			slices.Contains(config.RequiredDefinitions, def) {
			//include the definition.
			builder.WriteString(def.Code)
		}
	}

	builder.WriteString(base[prevDefEnd:])

	return builder.String(), nil
}

func IsBuiltinFeatureName(name string) bool {
	return slices.Contains(BUILTIN_FEATURE_NAMES, name)
}

func IsBuiltinCommandName(name string) bool {
	return slices.Contains(COMMAND_NAMES, name)
}

func GetBuiltinCommandDefinition(name string) (Definition, bool) {
	for _, def := range BUILTIN_DEFINITIONS {
		if def.Name == name && def.Kind == CommandDefinition {
			return def, true
		}
	}

	return Definition{}, false
}

func GetBuiltinFeatureDefinition(name string) (Definition, bool) {
	for _, def := range BUILTIN_DEFINITIONS {
		if def.Name == name && def.Kind == FeatureDefinition {
			return def, true
		}
	}

	return Definition{}, false
}
