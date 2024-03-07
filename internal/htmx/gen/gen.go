package gen

import (
	"strings"

	"github.com/inoxlang/inox/internal/htmx"
)

type Config struct {
	Extensions []string
}

// Generate generates a hyperscript.js file containing only the extensions listed in $config.
func Generate(config Config) (string, error) {

	w := strings.Builder{}

	w.WriteString("//included extensions: [")
	for i, ext := range config.Extensions {
		if i != 0 {
			w.WriteString(", ")
		}
		w.WriteString(ext)
	}

	w.WriteString("]\n\n")
	w.WriteString(htmx.HTMX_JS)

	for _, ext := range config.Extensions {
		w.WriteByte('\n')
		w.WriteString(htmx.EXTENSIONS[ext].Code)
	}

	return w.String(), nil
}
