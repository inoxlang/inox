package hsgen

import (
	_ "embed"
	"regexp"
)

var (
	//go:embed hyperscript.0.9.12.js
	HYPERSCRIPT_0_9_12_JS string

	ADD_COMMAND_LEN         = len("parser.addCommand")
	ADD_COMMAND_START_REGEX = regexp.MustCompile(`parser\.addCommand\(`)

	COMMAND_DEFINITION_REGIONS = map[string]CommandDefinition{}
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
		COMMAND_DEFINITION_REGIONS[def.CommandName] = def
	}

	println("1")
}

func Generate(config Config) (string, error) {
	return "", nil
}
