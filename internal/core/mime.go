package core

import (
	"strings"

	"github.com/inoxlang/inox/internal/mimeconsts"
)

type Mimetype string

func (mt Mimetype) WithoutParams() Mimetype {
	return Mimetype(strings.Split(string(mt), ";")[0])
}

func (mt Mimetype) UnderlyingString() string {
	return string(mt)
}

func GetMimeTypeFromExtension(extensionWithDot string) (Mimetype, bool) {
	m, ok := mimeconsts.FILE_EXTENSION_TO_MIMETYPE[extensionWithDot]
	if !ok {
		return "", false
	}
	return Mimetype(m), true
}
