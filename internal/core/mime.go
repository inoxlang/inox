package core

import (
	"mime"
	"strings"
)

// A Mimetype represents a MIME type, it can include parameters. Mimetype implements Value.
type Mimetype string

// MimeTypeFrom checks that s is a valid mime type and returns a normalized Mimetype.
func MimeTypeFrom(s string) (Mimetype, error) {
	mtype, params, err := mime.ParseMediaType(s)
	if err != nil {
		return "", err
	}

	return Mimetype(mime.FormatMediaType(mtype, params)), nil
}

func (mt Mimetype) WithoutParams() Mimetype {
	before, _, found := strings.Cut(string(mt), ";")
	if !found {
		return mt
	}
	return Mimetype(strings.TrimSpace(before))
}

func (mt Mimetype) UnderlyingString() string {
	return string(mt)
}

func GetMimeTypeFromExtension(extensionWithDot string) (Mimetype, bool) {
	mimeType := mime.TypeByExtension(extensionWithDot)
	if mimeType == "" {
		return "", false
	}
	return Mimetype(mimeType), true
}
