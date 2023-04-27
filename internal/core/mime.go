package internal

import (
	"strings"
)

//TODO: support structured syntax suffixes

const (
	ANY_CTYPE              = "*/*"
	JSON_CTYPE             = "application/json"
	IXON_CTYPE             = "application/ixon"
	APP_YAML_CTYPE         = "application/yaml" //https://datatracker.ietf.org/doc/draft-ietf-httpapi-yaml-mediatypes/
	TEXT_YAML_CTYPE        = "text/yaml"
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
	".yaml": APP_YAML_CTYPE,
	".yml":  APP_YAML_CTYPE,
	".css":  CSS_CTYPE,
	".js":   JS_CTYPE,
	".html": HTML_CTYPE,
	".htm":  HTML_CTYPE,
	".txt":  PLAIN_TEXT_CTYPE,
	".md":   PLAIN_TEXT_CTYPE,
}
