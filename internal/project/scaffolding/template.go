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

//go:embed base.css
var BASE_CSS_STYLESHEET string

//go:embed htmx-1.9.9.min.js
var HTMX_MIN_JS string

//go:embed templates/**
var TEMPLATE_FILES embed.FS

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
				case "base.css":
					content = []byte(BASE_CSS_STYLESHEET)
				}
			}

			return util.WriteFile(fls, pathInFs, content, fs_ns.DEFAULT_FILE_FMODE)
		}
	})
}
