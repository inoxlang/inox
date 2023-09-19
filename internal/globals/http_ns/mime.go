package http_ns

import (
	"fmt"
	"mime"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"
)

func Mime_(ctx *core.Context, arg core.Str) (core.Mimetype, error) {
	switch arg {
	case "json":
		arg = mimeconsts.JSON_CTYPE
	case "yaml":
		arg = mimeconsts.APP_YAML_CTYPE
	case "text":
		arg = mimeconsts.PLAIN_TEXT_CTYPE
	}

	_, _, err := mime.ParseMediaType(string(arg))
	if err != nil {
		return "", fmt.Errorf("'%s' is not a MIME type: %s", arg, err.Error())
	}

	return core.Mimetype(arg), nil
}
