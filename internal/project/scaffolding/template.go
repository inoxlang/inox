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
)

const (
	MINIMAL_WEB_APP_TEMPLATE_NAME = "web-app-min"
)

var (
	//go:embed templates/**
	TEMPLATE_FILES embed.FS

	//go:embed common/**
	COMMON_FILES embed.FS

	BASE_CSS_STYLESHEET         string
	INOX_JS                     string
	HTMX_MIN_JS                 string
	PREACT_SIGNALS_JS           string
	SURREAL_CSS_INLINE_SCOPE_JS string

	FULL_INOX_JS string
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
		case "preact-signals.js":
			PREACT_SIGNALS_JS = string(content)
		case "base.css":
			BASE_CSS_STYLESHEET = string(content)
		case "surreal-css-inline-scope.js":
			SURREAL_CSS_INLINE_SCOPE_JS = string(content)
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

	FULL_INOX_JS = strings.Join(parts, "\n")
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
					content = []byte(HTMX_MIN_JS)
				case "inox.js":
					content = []byte(FULL_INOX_JS)
				case "base.css":
					content = []byte(BASE_CSS_STYLESHEET)
				}
			}

			return util.WriteFile(fls, pathInFs, content, fs_ns.DEFAULT_FILE_FMODE)
		}
	})
}
