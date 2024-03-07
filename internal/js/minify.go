package js

import (
	"bytes"
	"io"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

var (
	minifier = minify.New()
)

func MustMinify(input string, params map[string]string) string {
	return utils.Must(Minify(input, params))
}

func Minify(input string, params map[string]string) (string, error) {
	output := bytes.NewBuffer(nil)
	err := js.Minify(minifier, output, strings.NewReader(input), params)
	if err != nil {
		return "", err
	}
	return output.String(), nil
}

func MinifyStream(input io.Reader, writer io.Writer, params map[string]string) error {
	return js.Minify(minifier, writer, input, params)
}

func ReadMinify(output io.Writer, input io.Reader, params map[string]string) error {
	return js.Minify(minifier, output, input, params)
}
