package scaffolding

import (
	"bytes"
	"embed"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/js"
)

const (
	MINIMAL_WEB_APP_TEMPLATE_NAME = "web-app-min"
)

var (
	//go:embed templates/**
	TEMPLATE_FILES embed.FS

	//go:embed common/**
	COMMON_FILES embed.FS

	BASE_CSS_STYLESHEET string
	INOX_JS             string //inox.js without any other library
	HTMX_MIN_JS         string

	JSON_FORM_EXTENSION_JS string
	JSON_FORM_EXT_MINIFIED string

	PREACT_SIGNALS_JS       string
	PREACT_SIGNALS_MINIFIED string

	SURREAL_CSS_INLINE_SCOPE_JS       string
	SURREAL_CSS_INLINE_SCOPE_MINIFIED string

	HTMX_MIN_JS_PACKAGE string

	INOX_JS_PACKAGE          string
	INOX_JS_PACKAGE_MINIFIED string
)

func init() {
	err := fs.WalkDir(COMMON_FILES, "common", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		content, err := COMMON_FILES.ReadFile(path)
		if err != nil {
			return err
		}

		switch filepath.Base(path) {
		case "inox.js":
			INOX_JS = string(content)
		case "htmx-1.9.9.min.js":
			HTMX_MIN_JS = string(content)
		case "json-form-extension.js":
			JSON_FORM_EXTENSION_JS = string(content)
			JSON_FORM_EXT_MINIFIED = js.MustMinify(JSON_FORM_EXTENSION_JS, nil)
		case "preact-signals.js":
			PREACT_SIGNALS_JS = string(content)
			PREACT_SIGNALS_MINIFIED = js.MustMinify(PREACT_SIGNALS_JS, nil)
		case "base.css":
			BASE_CSS_STYLESHEET = string(content)
		case "surreal-css-inline-scope.js":
			SURREAL_CSS_INLINE_SCOPE_JS = string(content)
			SURREAL_CSS_INLINE_SCOPE_MINIFIED = js.MustMinify(SURREAL_CSS_INLINE_SCOPE_JS, nil)
		}

		return nil
	})

	if err != nil {
		panic(err)
	}

	parts := []string{
		"{\n" + INOX_JS + "\n}\n",
		"{\n" + PREACT_SIGNALS_JS + "\n}\n",
		SURREAL_CSS_INLINE_SCOPE_JS,
	}

	INOX_JS_PACKAGE = strings.Join(parts, "\n")
	INOX_JS_PACKAGE_MINIFIED = js.MustMinify(INOX_JS_PACKAGE, nil)

	HTMX_MIN_JS_PACKAGE = HTMX_MIN_JS + "\n" + JSON_FORM_EXT_MINIFIED
}

func WriteTemplate(name string, fls afs.Filesystem) error {
	var DIR = "templates/" + name

	return fs.WalkDir(TEMPLATE_FILES, DIR, func(path string, d fs.DirEntry, err error) error {
		pathInFs := strings.TrimPrefix(path, DIR)
		if pathInFs == "" {
			return nil
		}
		if d.IsDir() {
			return fls.MkdirAll(pathInFs, fs_ns.DEFAULT_DIR_FMODE)
		} else {
			content, err := TEMPLATE_FILES.ReadFile(path)
			if err != nil {
				return err
			}

			//special filenames
			if len(content) < 20 && bytes.Contains(content, []byte("[auto]")) {
				switch filepath.Base(pathInFs) {
				case "htmx.min.js":
					content = []byte(HTMX_MIN_JS_PACKAGE)
				case "inox.min.js":
					content = []byte(INOX_JS_PACKAGE_MINIFIED)
				case "base.css":
					content = []byte(BASE_CSS_STYLESHEET)
				}
			}

			return util.WriteFile(fls, pathInFs, content, fs_ns.DEFAULT_FILE_FMODE)
		}
	})
}
