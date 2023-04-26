package internal

import (
	"strings"
)

const (
	ANY_CTYPE              = "*/*"
	JSON_CTYPE             = "application/json"
	IXON_CTYPE             = "application/ixon"
	HTML_CTYPE             = "text/html"
	CSS_CTYPE              = "text/css"
	JS_CTYPE               = "text/javascript" //https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types#textjavascript
	PLAIN_TEXT_CTYPE       = "text/plain"
	EVENT_STREAM_CTYPE     = "text/event-stream"
	APP_OCTET_STREAM_CTYPE = "application/octet-stream"
)

type Mimetype string

func (mt Mimetype) WithoutParams() Mimetype {
	return Mimetype(strings.Split(string(mt), ";")[0])
}

func (mt Mimetype) UnderlyingString() string {
	return string(mt)
}

var FILE_EXTENSION_TO_MIMETYPE = map[string]Mimetype{
	".json": JSON_CTYPE,
	".css":  CSS_CTYPE,
	".js":   JS_CTYPE,
	".html": HTML_CTYPE,
	".htm":  HTML_CTYPE,
	".txt":  PLAIN_TEXT_CTYPE,
	".md":   PLAIN_TEXT_CTYPE,
}
