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

	ADD_COMMAND_LEN         = len("parser.addCommand")
	ADD_COMMAND_START_REGEX = regexp.MustCompile(`parser\.addCommand\(`)

	COMMAND_DEFINITIONS = []CommandDefinition{}
	COMMAND_NAMES       []string
)

type Config struct {
	Commands []string
}

type CommandDefinition struct {
	CommandName string
	Start       int
	End         int
	Code        string
}

func init() {
	hyperscript := HYPERSCRIPT_0_9_12_JS

	positions := ADD_COMMAND_START_REGEX.FindAllStringIndex(hyperscript, -1)

	//Find all command definition regions.
	for _, pos := range positions {
		start := pos[0]
		def := GetCommandDefinition(start, hyperscript)
		COMMAND_DEFINITIONS = append(COMMAND_DEFINITIONS, def)
		COMMAND_NAMES = append(COMMAND_NAMES, def.CommandName)
	}
}

// Generate generates a subset of hyperscript.js that does not contain the command definitions listed in the configuration.
func Generate(config Config) (string, error) {

	base := HYPERSCRIPT_0_9_12_JS
	prevDefEnd := 0

	builder := strings.Builder{}

	for _, def := range COMMAND_DEFINITIONS {
		beforeDefinition := base[prevDefEnd:def.Start]
		builder.WriteString(beforeDefinition)

		prevDefEnd = def.End

		if slices.Contains(config.Commands, def.CommandName) {
			//include the definition.
			builder.WriteString(def.Code)
		}
	}

	builder.WriteString(base[prevDefEnd:])

	return builder.String(), nil
}
