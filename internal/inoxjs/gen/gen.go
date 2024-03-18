package gen

import (
	"strings"

	"github.com/inoxlang/inox/internal/inoxjs"
)

type Config struct {
	Libraries []string
}

// Generate generates a inox.gen.js file containing only the libraries listed in $config.
func Generate(config Config) (string, error) {

	w := strings.Builder{}

	w.WriteString("//included libraries: [")
	for i, ext := range config.Libraries {
		if i != 0 {
			w.WriteString(", ")
		}
		w.WriteString(ext)
	}

	w.WriteString("]\n")

	for _, libName := range config.Libraries {
		w.WriteByte('\n')
		w.WriteString(inoxjs.LIBRARIES[libName])
	}

	return w.String(), nil
}
