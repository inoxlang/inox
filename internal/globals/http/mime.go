package internal

import (
	"fmt"
	"mime"

	core "github.com/inoxlang/inox/internal/core"
)

func Mime_(ctx *core.Context, arg core.Str) (core.Mimetype, error) {
	switch arg {
	case "json":
		arg = core.JSON_CTYPE
	case "yaml":
		arg = core.APP_YAML_CTYPE
	case "text":
		arg = core.PLAIN_TEXT_CTYPE
	}

	_, _, err := mime.ParseMediaType(string(arg))
	if err != nil {
		return "", fmt.Errorf("'%s' is not a MIME type: %s", arg, err.Error())
	}

	return core.Mimetype(arg), nil
}
