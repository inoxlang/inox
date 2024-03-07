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
	"github.com/inoxlang/inox/internal/project/layout"
)

const (
	MINIMAL_WEB_APP_TEMPLATE_NAME = "web-app-min"
)

var (
	//go:embed templates/**
	TEMPLATE_FILES embed.FS

	//go:embed common/**
	COMMON_FILES embed.FS

	MAIN_CSS_STYLESHEET                      string
	MAIN_CSS_STYLESHEET_WITH_TAILWIND_IMPORT string

	//Inox.js package

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
		basename := filepath.Base(path)

		switch basename {
		case "main.css":
			MAIN_CSS_STYLESHEET = string(content)
			MAIN_CSS_STYLESHEET_WITH_TAILWIND_IMPORT = layout.TAILWIND_IMPORT + "\n\n" + MAIN_CSS_STYLESHEET
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
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
				case "main.css":
					content = []byte(MAIN_CSS_STYLESHEET_WITH_TAILWIND_IMPORT)
				case layout.TAILWIND_FILENAME:
					content = []byte(layout.TAILWIND_CSS_STYLESHEET_EXPLANATION)
				}
			}

			return util.WriteFile(fls, pathInFs, content, fs_ns.DEFAULT_FILE_FMODE)
		}
	})
}
