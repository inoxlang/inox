package codecompletion

import (
	httpspec "github.com/inoxlang/inox/internal/globals/http_ns/spec"
)

type InputData struct {
	StaticFileURLPaths []string      //examples: /index.js, /index.css
	ServerAPI          *httpspec.API //optional
}
