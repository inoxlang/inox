package core

import (
	"mime"
	"strings"
)

type Mimetype string

func (mt Mimetype) WithoutParams() Mimetype {
	return Mimetype(strings.Split(string(mt), ";")[0])
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
