package mimeconsts

import "mime"

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
	INOX_CTYPE             = "text/inox"
	PLAIN_TEXT_CTYPE       = "text/plain"
	EVENT_STREAM_CTYPE     = "text/event-stream"
	APP_OCTET_STREAM_CTYPE = "application/octet-stream"
	MULTIPART_FORM_DATA    = "multipart/form-data"
)

func init() {
	mime.AddExtensionType(".txt", PLAIN_TEXT_CTYPE)
}

// IsMimeTypeForExtension returns true if ext corresponds to mimetype.
func IsMimeTypeForExtension(mimetype string, ext string) bool {
	actual := TypeByExtensionWithoutParams(mimetype)
	return mimetype == actual
}

func TypeByExtensionWithoutParams(ext string) string {
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return ""
	}
	mimeType, _, _ = mime.ParseMediaType(mimeType)
	return mimeType
}
