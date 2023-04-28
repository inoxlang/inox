package internal

import (
	"errors"
	"fmt"

	"github.com/muesli/termenv"

	core "github.com/inoxlang/inox/internal/core"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

type REPLConfiguration struct {
	builtinCommands   []string
	trustedCommands   []string
	additionalGlobals map[string]core.Value
	prompt            *core.List

	handleSignals bool

	PrintingConfig
}

// MakeREPLConfiguration constructs a REPLConfiguration from an Object
func MakeREPLConfiguration(obj *core.Object) (REPLConfiguration, error) {
	config := REPLConfiguration{
		handleSignals:  true,
		PrintingConfig: GetPrintingConfig(),
	}

	//use state.Out instead of stdout ?

	for k, v := range obj.EntryMap() {
		switch k {
		case "builtin-commands":
			const BUILTIN_COMMAND_LIST_ERR = "invalid configuration: .builtin-commands should be a list of identifiers"

			list, isList := v.(*core.List)
			if !isList {
				return config, errors.New(BUILTIN_COMMAND_LIST_ERR)
			}
			for _, cmd := range list.GetOrBuildElements(nil) {
				ident, ok := cmd.(core.Identifier)
				if !ok {
					return config, errors.New(BUILTIN_COMMAND_LIST_ERR)
				}
				config.builtinCommands = append(config.builtinCommands, string(ident))
			}
		case "trusted-commands":
			const ALIASED_COMMAND_LIST_ERR = "invalid configuration: .trusted-commands should be a list of identifiers"

			list, isList := v.(*core.List)
			if !isList {
				return config, errors.New(ALIASED_COMMAND_LIST_ERR)
			}
			for _, cmd := range list.GetOrBuildElements(nil) {
				ident, ok := cmd.(core.Identifier)
				if !ok {
					return config, errors.New(ALIASED_COMMAND_LIST_ERR)
				}
				config.trustedCommands = append(config.trustedCommands, string(ident))
			}
		case "globals":
			const GLOBALS_ERR = "invalid configuration: .globals should be an object"
			obj, ok := v.(*core.Object)
			config.additionalGlobals = make(map[string]core.Value)
			if !ok {
				return config, errors.New(GLOBALS_ERR)
			}

			for k, v := range obj.EntryMap() {
				config.additionalGlobals[k] = v
			}
		case "prompt":
			const PROMPT_CONFIG_ERR = "invalid configuration: prompt should be a list"

			list, isList := v.(*core.List)
			if !isList {
				return config, errors.New(PROMPT_CONFIG_ERR)
			}
			for _, part := range list.GetOrBuildElements(nil) {

				if list, isList := part.(*core.List); isList {
					if list.Len() != 3 {
						return config, fmt.Errorf(
							"invalid configuration: parts of type List should be three element long: [<desc.>, <color identifier>, <color identifier>]",
						)
					}
					part = list.At(nil, 0)
				}

				switch p := part.(type) {
				case core.Str:
				// case Identifier:
				// 	switch part {
				// 	case "cwd":
				// 	default:
				// 		return config, fmt.Errorf("invalid configuration: invalid part in prompt configuration: %s is not valid identifier", p)
				// 	}
				case core.AstNode:
				default:
					return config, fmt.Errorf("invalid configuration: invalid part in prompt configuration: type %T", p)
				}
			}
			config.prompt = list
		}
	}

	return config, nil
}

type PrintingConfig struct {
	defaultFgColor                 core.Color
	defaultFgColorSequence         []byte
	backgroundColor                core.Color
	defaultBackgroundColorSequence []byte

	prettyPrintConfig *core.PrettyPrintConfig
}

func (c PrintingConfig) PrettyPrintConfig() *core.PrettyPrintConfig {
	config := *defaultPrettyPrintConfig
	return &config
}

func (c PrintingConfig) Colorized() bool {
	return c.prettyPrintConfig.Colorize
}

func (c PrintingConfig) IsLight() bool {
	return !c.backgroundColor.IsDarkBackgroundColor()
}

func GetPrintingConfig() PrintingConfig {
	config := PrintingConfig{}
	config.defaultFgColor = core.ColorFromTermenvColor(termenv.ForegroundColor(), core.ColorFromTermenvColor(termenv.ANSIWhite))
	config.defaultFgColorSequence = config.defaultFgColor.GetAnsiEscapeSequence(false)
	config.backgroundColor = core.ColorFromTermenvColor(termenv.BackgroundColor(), core.ColorFromTermenvColor(termenv.ANSIBlack))
	config.defaultBackgroundColorSequence = config.backgroundColor.GetAnsiEscapeSequence(true)

	prettyPrintConfig := *defaultPrettyPrintConfig
	config.prettyPrintConfig = &prettyPrintConfig

	if config.IsLight() {
		config.prettyPrintConfig.Colors = &pprint.DEFAULT_LIGHTMODE_PRINT_COLORS
	}

	return config
}
