package binary

import (
	"fmt"
	"path/filepath"
	"runtime"
)

var INOX_BINARY_ARCHIVE_GLOB_MATRIX = map[string]map[string]string{
	"linux": {
		"amd64": ".*linux-amd64.tar.gz",
	},
}

type assetInfo struct {
	Name string `json:"name"`
}
