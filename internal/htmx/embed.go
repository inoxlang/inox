package htmx

import (
	"embed"
	"path/filepath"
	"slices"
	"strings"

	"github.com/inoxlang/inox/internal/js"
)

var (
	//go:embed htmx.1.9.10.js
	HTMX_JS          string
	MINIFIED_HTMX_JS string

	//go:embed extensions/*
	extensionsFS embed.FS
)

func ReadEmbedded() {

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

		extension := EXTENSIONS[extName]

		//The extension's documentation may be set.

		extension.Name = extName
		extension.Code = sourceCodeString
		extension.MinifiedCode = js.MustMinify(sourceCodeString, nil)
		EXTENSIONS[extName] = extension
	}
}

func GetExtensionInfoByPrefix(prefix string) (extensions []Extension) {
	lowercasePrefix := strings.ToLower(prefix)

	for name, extension := range EXTENSIONS {
		if strings.HasPrefix(name, lowercasePrefix) {
			extensions = append(extensions, extension)
		}
	}

	slices.SortFunc(extensions, func(a, b Extension) int {
		return strings.Compare(a.Name, b.Name)
	})

	return extensions
}
