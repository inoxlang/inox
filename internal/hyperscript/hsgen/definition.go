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

type Definition struct {
	Kind  DefinitionType
	Name  string
	Start int
	End   int
	Code  string
}

type DefinitionType int

const (
	CommandDefinition DefinitionType = iota
	FeatureDefinition
)

func GetDefinition(definitionStart int, kind DefinitionType, definitions string) Definition {
	commandNameStart := definitionStart
	switch kind {
	case CommandDefinition:
		commandNameStart += ADD_COMMAND_LEN + (1 /* '('*/)
	case FeatureDefinition:
		commandNameStart += ADD_FEATURE_LEN + (1 /* '('*/)
	}

	definitionEnd := commandNameStart
	subString := definitions[commandNameStart:]

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

		if len(s) == 1 {
			if s[0] == '(' {
				parenCount++
			} else if s[0] == ')' {
				parenCount--
			}
		}
	}

	_, s := lexer.Next()
	if len(s) == 1 && s[0] == ';' {
		definitionEnd++
	}

	region := Definition{
		Kind:  kind,
		Name:  commandName,
		Start: definitionStart,
		End:   definitionEnd,
		Code:  definitions[definitionStart:definitionEnd],
	}

	if doFixReplacement {
		region.Code = strings.ReplaceAll(region.Code, BUGGY_REGEX_REPLACEMENT, BUGGY_REGEX)
	}

	return region
}
