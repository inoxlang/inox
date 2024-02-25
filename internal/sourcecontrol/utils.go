package sourcecontrol

import (
	"path/filepath"
)

func toCleanRelativePath(path string) string {
	path = filepath.Clean(path)

	if path[0] != '/' { //Already relative.
		return path
	}

	if path == "/" {
		return "."
	}

	return path[1:]
}

func toCleanAbsolutePath(path string) string {
	path = filepath.Clean(path)

	if path[0] == '/' { //Already absolute.
		return path
	}

	if path == "." {
		return "/"
	}

	return "/" + path
}
