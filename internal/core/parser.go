package core

import (
	"encoding/json"
	"errors"

	yamlLex "github.com/goccy/go-yaml/lexer"
	yamlParse "github.com/goccy/go-yaml/parser"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	parsers = map[Mimetype] /* no params */ StatelessParser{}
	_       = []StatelessParser{(*jsonParser)(nil)}
)

func init() {
	RegisterParser(mimeconsts.JSON_CTYPE, &jsonParser{})
	RegisterParser(mimeconsts.IXON_CTYPE, &inoxReprParser{})
	RegisterParser(mimeconsts.APP_YAML_CTYPE, &yamlParser{})
}

// A StatelessParser represents a parser for a data format such as JSON or YAML.
// Implementations are allowed to use caching internally.
type StatelessParser interface {
	Validate(ctx *Context, s string) bool

	// Parse parses a string in the data format supported by the parser and returns the resulting value.
	// Mutable returned values should never be stored to be returned later.
	Parse(ctx *Context, s string) (Serializable, error)
}

func RegisterParser(mime Mimetype, p StatelessParser) {
	if _, ok := parsers[mime]; ok {
		panic(errors.New("a parser is already registered for mime " + string(mime)))
	}
	parsers[mime] = p
}

func GetParser(mime Mimetype) (StatelessParser, bool) {
	p, ok := parsers[mime.WithoutParams()]
	return p, ok
}

type jsonParser struct {
}

func (p *jsonParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}
	return json.Valid(utils.StringAsBytes(s))

}
func (p *jsonParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	var jsonVal any
	err := json.Unmarshal(utils.StringAsBytes(s), &jsonVal)
	if err != nil {
		return nil, err
	}
	//TODO: use ParseJSONRepresentation (add tests before change)
	return ConvertJSONValToInoxVal(jsonVal, false), nil
}

type inoxReprParser struct {
}

func (p *inoxReprParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	_, err := ParseRepr(ctx, utils.StringAsBytes(s))
	return err == nil

}
func (p *inoxReprParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	return ParseRepr(ctx, utils.StringAsBytes(s))
}

type yamlParser struct {
}

func (p *yamlParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	tokens := yamlLex.Tokenize(s)
	_, err := yamlParse.Parse(tokens, yamlParse.ParseComments)
	return err == nil
}

func (p *yamlParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	tokens := yamlLex.Tokenize(s)
	yml, err := yamlParse.Parse(tokens, 0)

	if err != nil {
		return nil, err
	}
	return ConvertYamlParsedFileToInoxVal(ctx, yml, false), nil
}

type ulidParser struct {
}

func (p *ulidParser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	_, err := ParseULID(s)
	return err == nil
}

func (p *ulidParser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	return ParseULID(s)
}

type uuidv4Parser struct {
}

func (p *uuidv4Parser) Validate(ctx *Context, s string) bool {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		panic(ErrTestedStringTooLarge)
	}

	_, err := ParseUUIDv4(s)
	return err == nil
}

func (p *uuidv4Parser) Parse(ctx *Context, s string) (Serializable, error) {
	if len(s) > DEFAULT_MAX_TESTED_STRING_BYTE_LENGTH {
		return nil, ErrTestedStringTooLarge
	}

	return ParseUUIDv4(s)
}
