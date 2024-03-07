package htmx

import (
	"embed"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/js"
)

var (
	//go:embed htmx.1.9.10.js
	HTMX_JS          string
	MINIFIED_HTMX_JS string

	//go:embed extensions/*
	extensionsFS embed.FS

	EXTENSIONS          = map[string]string{}
	MINIFIED_EXTENSIONS = map[string]string{}
)

func ReadEmbedded() {
	if len(EXTENSIONS) != 0 {
		return
	}

	MINIFIED_HTMX_JS = js.MustMinify(HTMX_JS, nil)

	entries, err := extensionsFS.ReadDir("extensions")
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		entryName := entry.Name()
		ext := filepath.Ext(entryName)
		extName := strings.TrimSuffix(entryName, ext)

		sourceCode, err := extensionsFS.ReadFile("extensions/" + entryName)
		if err != nil {
			panic(err)
		}

		sourceCodeString := string(sourceCode)

		EXTENSIONS[extName] = sourceCodeString
		MINIFIED_EXTENSIONS[extName] = js.MustMinify(sourceCodeString, nil)
	}
}
