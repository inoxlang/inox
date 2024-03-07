package css

import (
	"io"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

var minifier = minify.New()

func MinifyStream(input io.Reader, output io.Writer) error {
	return css.Minify(minifier, output, input, nil)
}
