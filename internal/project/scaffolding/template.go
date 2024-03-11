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

	//go:embed common/main.css
	MAIN_CSS string

	//go:embed common/reset.css
	RESET_CSS string

	//go:embed common/vars.css
	VARS_CSS string
)

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
					content = []byte(MAIN_CSS)
				case "reset.css":
					content = []byte(RESET_CSS)
				case "vars.css":
					content = []byte(VARS_CSS)
				case layout.UTILITY_CLASSES_FILENAME:
					content = []byte(layout.UTILITY_CLASSES_STYLESHEET_EXPLANATION)
				}
			}

			return util.WriteFile(fls, pathInFs, content, fs_ns.DEFAULT_FILE_FMODE)
		}
	})
}
