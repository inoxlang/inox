package htmx

import (
	"embed"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/inoxlang/inox/internal/js"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed htmx.1.9.10.js
	HTMX_JS          string
	MINIFIED_HTMX_JS string

	//go:embed extensions/*
	extensionsFS embed.FS

	//go:embed headers.yaml
	headersInfo string
)

func Load() {

	MINIFIED_HTMX_JS = js.MustMinify(HTMX_JS, nil)

	extensionEntries, err := extensionsFS.ReadDir("extensions")
	if err != nil {
		panic(err)
	}

	for _, entry := range extensionEntries {
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

	//Get information about headers.
	err = yaml.Unmarshal(utils.StringAsBytes(headersInfo), &HEADERS)
	if err != nil {
		panic(err)
	}

	for name, header := range HEADERS.Request {
		header.Name = name
		HEADERS.Request[name] = header
	}

	for name, header := range HEADERS.Response {
		header.Name = name
		HEADERS.Response[name] = header
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
