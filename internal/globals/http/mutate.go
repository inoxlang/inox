package internal

import (
	"errors"
	"fmt"
	"io"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
)

func HttpPost(ctx *core.Context, args ...core.Value) (*HttpResponse, error) {
	return _httpPostPatch(ctx, false, args...)
}

func HttpPatch(ctx *core.Context, args ...core.Value) (*HttpResponse, error) {
	return _httpPostPatch(ctx, true, args...)
}

func _httpPostPatch(ctx *core.Context, isPatch bool, args ...core.Value) (*HttpResponse, error) {
	var contentType core.Mimetype
	var u core.URL
	var body io.Reader
	var requestOptionArgs []core.Value

	for _, arg := range args {
		switch argVal := arg.(type) {
		case core.URL:
			if u != "" {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("url")
			}
			u = argVal
		case core.Mimetype:
			if contentType != "" {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("mime tyoe")
			}
			contentType = argVal
		case core.Readable:
			if body != nil {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("body")
			}
			body = argVal.Reader()
		case *core.List:
			if body != nil {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("body")
			}
			jsonString := core.ToJSON(ctx, argVal)
			body = strings.NewReader(string(jsonString))
		case *core.Object:
			if body == nil {
				jsonString := core.ToJSON(ctx, argVal)
				body = strings.NewReader(string(jsonString))
			} else {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("body")
			}
		case core.Option, core.QuantityRange:
			requestOptionArgs = append(requestOptionArgs, argVal)
		default:
			return nil, fmt.Errorf("only an URL argument is expected, not a(n) %T ", arg)
		}
	}

	//checks

	if u == "" {
		return nil, errors.New(MISSING_URL_ARG)
	}

	client, opts, err := getClientAndOptions(ctx, u, requestOptionArgs...)
	if err != nil {
		return nil, err
	}

	//

	method := "POST"
	if isPatch {
		method = "PATCH"
	}
	req, err := client.MakeRequest(ctx, method, u, body, string(contentType), opts)
	if err != nil {
		return nil, err
	}
	return client.DoRequest(ctx, req)
}

func HttpDelete(ctx *core.Context, args ...core.Value) (*HttpResponse, error) {
	var u core.URL
	var requestOptionArgs []core.Value

	for _, arg := range args {
		switch argVal := arg.(type) {
		case core.URL:
			if u != "" {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("url")
			}
			u = argVal
		case core.Option, core.QuantityRange:
			requestOptionArgs = append(requestOptionArgs, argVal)
		default:
			return nil, fmt.Errorf("only an core.URL argument is expected, not a(n) %T ", arg)
		}
	}

	//checks

	if u == "" {
		return nil, errors.New(MISSING_URL_ARG)
	}

	client, opts, err := getClientAndOptions(ctx, u, requestOptionArgs...)
	if err != nil {
		return nil, err
	}

	req, err := client.MakeRequest(ctx, "DELETE", u, nil, "", opts)
	if err != nil {
		return nil, err
	}
	return client.DoRequest(ctx, req)
}
