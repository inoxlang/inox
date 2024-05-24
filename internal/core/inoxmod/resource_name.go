package inoxmod

import (
	"net/url"
	"path/filepath"
)

type ResourceName interface {
	ResourceName() string
	IsURL() bool
	IsPath() bool
}

func isPathRelative(path string) bool {
	return path[0] != '/'
}

func isPathAbsolute(path string) bool {
	return path[0] == '/'
}

func isPathDirPath(path string) bool {
	return path[len(path)-1] == '/'
}

func getParentUrlDir(urlString string) (string, bool, error) {
	u, err := url.ParseRequestURI(urlString)
	if err != nil {
		return "", false, err
	}
	if u.Path == "" || u.Path == "/" {
		return "", false, nil
	}

	path := filepath.Dir(u.Path)
	if path[len(path)-1] != '/' {
		path += "/"
	}

	u.Path = path
	return u.String(), true, nil
}
