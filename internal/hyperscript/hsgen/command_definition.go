package hsgen

import (
	"errors"
	"io"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

const (
	BUGGY_REGEX = `/(?:(^|#|\.)([^#\. ]+))/g` //This reges causes a tokenization bug.
)

var (
	BUGGY_REGEX_REPLACEMENT = strings.ReplaceAll(BUGGY_REGEX, `/`, `"`) //the replacement should have the same length.
)

func GetCommandDefinition(definitionStart int, definitions string) CommandDefinition {
	commandNameStart := definitionStart + ADD_COMMAND_LEN + (1 /* '('*/)
	definitionEnd := commandNameStart

	afterDefinitionEnd := min(len(definitions), commandNameStart+10_000)
	subString := definitions[commandNameStart:afterDefinitionEnd]

	doFixReplacement := false

	if strings.Contains(subString, BUGGY_REGEX) {
		doFixReplacement = true
		subString = strings.ReplaceAll(subString, BUGGY_REGEX, BUGGY_REGEX_REPLACEMENT)
	}

	lexer := js.NewLexer(parse.NewInputString(subString))

	_, commandNameStringLiteral := lexer.Next()

	commandName := strings.Trim(string(commandNameStringLiteral), `"`)
	definitionEnd += len(commandNameStringLiteral)

	var parenCount = 1 //'(' after parser.addCommand

	var prev []byte

	for parenCount > 0 { //Stop at end of definition.
		_, s := lexer.Next()
		definitionEnd += len(s)

		err := lexer.Err()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}

		prev = s
		_ = prev

		if len(s) == 1 {
			if s[0] == '(' {
				parenCount++
			} else if s[0] == ')' {
				parenCount--
			}
		}
	}

	region := CommandDefinition{
		CommandName: commandName,
		Start:       definitionStart,
		End:         definitionEnd,
		Code:        definitions[definitionStart:definitionEnd],
	}

	if doFixReplacement {
		region.Code = strings.ReplaceAll(region.Code, BUGGY_REGEX_REPLACEMENT, BUGGY_REGEX)
	}

	return region
}
