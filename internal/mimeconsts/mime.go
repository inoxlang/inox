package mimeconsts

import "mime"

//TODO: support structured syntax suffixes

const (
	ANY_CTYPE  = "*/*"
	JSON_CTYPE = "application/json"

	//https://datatracker.ietf.org/doc/draft-ietf-httpapi-yaml-mediatypes/
	/* quote:
	"Deprecated alias names for this type: application/x-yaml, text/yaml,
	text/x-yaml. These names are used, but not registered."
	*/
	APP_YAML_CTYPE = "application/yaml"
	//TEXT_YAML_CTYPE        = "text/yaml"

	HTML_CTYPE             = "text/html"
	CSS_CTYPE              = "text/css"
	JS_CTYPE               = "text/javascript"  //https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types#textjavascript
	HYPERSCRIPT_CTYPE      = "text/hyperscript" //https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types#textjavascript
	INOX_CTYPE             = "text/inox"
	PLAIN_TEXT_CTYPE       = "text/plain"
	EVENT_STREAM_CTYPE     = "text/event-stream"
	APP_OCTET_STREAM_CTYPE = "application/octet-stream"
	MULTIPART_FORM_DATA    = "multipart/form-data"

	//images

	AVIF_CTYPE = "image/avif"
	GIF_CTYPE  = "image/gif"
	JPEG_CTYPE = "image/jpeg"
	SVG_CTYPE  = "image/svg+xml"
	WEBP_CTYPE = "image/webp"
	PNG_CTYPE  = "image/png"
)

var (
	COMMON_IMAGE_CTYPES = []string{AVIF_CTYPE, GIF_CTYPE, JPEG_CTYPE, SVG_CTYPE, WEBP_CTYPE, PNG_CTYPE}
)

func init() {
	mime.AddExtensionType(".txt", PLAIN_TEXT_CTYPE)
	mime.AddExtensionType(".yaml", APP_YAML_CTYPE)
	mime.AddExtensionType(".yml", APP_YAML_CTYPE)
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
