package projectserver

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

// normalizeURI turns URIs of the shape scheme:/path URIs into scheme:///path,
// it does not check if the URI is valid.
func normalizeURI[S ~string](uri S) S {
	scheme, afterColonSlash, ok := strings.Cut(string(uri), ":/")
	if !ok {
		//invalid URI
		return uri
	}
	if strings.HasPrefix(afterColonSlash, "//") {
		return uri //already normalized
	}
	return S(scheme + ":///" + afterColonSlash)
}

func getFileURI(path string, usingInoxFs bool) (defines.DocumentUri, error) {
	if path == "" {
		return "", errors.New("failed to get document URI: empty path")
	}
	if path[0] != '/' {
		return "", fmt.Errorf("failed to get document URI: path is not absolute: %q", path)
	}
	if usingInoxFs {
		return defines.DocumentUri(INOX_FS_SCHEME + "://" + path), nil
	}
	return defines.DocumentUri("file://" + path), nil
}

// getPath returns a clean path from a URI.
func getPath(uri defines.URI, usingInoxFS bool) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if usingInoxFS && u.Scheme != INOX_FS_SCHEME {
		return "", fmt.Errorf("%w, actual is: %s", ErrInoxURIExpected, string(uri))
	}
	if !usingInoxFS && u.Scheme != "file" {
		return "", fmt.Errorf("%w, actual is: %s", ErrFileURIExpected, string(uri))
	}
	return filepath.Clean(u.Path), nil
}

// getPath returns a clean path from a URI, it also checks that the file extension is `.ix` or `._hs`.
func getSupportedFilePath(uri defines.DocumentUri, usingInoxFs bool) (string, error) {
	u, err := url.Parse(string(uri))

	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if usingInoxFs && u.Scheme != INOX_FS_SCHEME {
		return "", fmt.Errorf("%w, URI is: %s", ErrInoxURIExpected, string(uri))
	}
	if !usingInoxFs && u.Scheme != "file" {
		return "", fmt.Errorf("%w, URI is: %s", ErrFileURIExpected, string(uri))
	}

	clean := filepath.Clean(u.Path)
	if !strings.HasSuffix(clean, inoxconsts.INOXLANG_FILE_EXTENSION) && !strings.HasSuffix(clean, hscode.FILE_EXTENSION) {
		return "", fmt.Errorf("unexpected file extension: '%s'", filepath.Ext(clean))
	}
	return clean, nil
}
