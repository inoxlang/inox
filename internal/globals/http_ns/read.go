package http_ns

import (
	"errors"
	"fmt"
	"io"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
)

func HttpGet(ctx *core.Context, u core.URL, args ...core.Value) (*HttpResponse, error) {
	var contentType core.Mimetype
	var requestOptionArgs []core.Value

	for _, arg := range args {
		switch argVal := arg.(type) {
		case core.URL:
			return nil, commonfmt.FmtErrArgumentProvidedAtLeastTwice("url")
		case core.Mimetype:
			if contentType != "" {
				return nil, commonfmt.FmtErrXProvidedAtLeastTwice("mime type")
			}
			contentType = argVal
		case core.Option, core.QuantityRange:
			requestOptionArgs = append(requestOptionArgs, argVal)
		default:
			return nil, fmt.Errorf("invalid argument, type = %T ", arg)
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

	req, err := client.MakeRequest(ctx, "GET", u, nil, string(contentType), opts)
	if err != nil {
		return nil, err
	}
	return client.DoRequest(ctx, req)
}

func HttpRead(ctx *core.Context, u core.URL, args ...core.Value) (result core.Value, finalErr error) {
	var contentType core.Mimetype
	var b []byte
	var requestOptionArgs []core.Value
	doParse := true
	validateRaw := false

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Mimetype:
			if contentType != "" {
				finalErr = commonfmt.FmtErrXProvidedAtLeastTwice("content type")
				return
			}
			contentType = v
		case core.Option:
			if v.Name == "raw" {
				if v.Value == core.True {
					doParse = false
				}
			} else {
				requestOptionArgs = append(requestOptionArgs, v)
			}
		case core.QuantityRange:
			requestOptionArgs = append(requestOptionArgs, v)
		default:
			return nil, fmt.Errorf("invalid argument %#v", arg)
		}
	}

	var httpGetArgs []core.Value
	if contentType != "" {
		httpGetArgs = append(httpGetArgs, contentType)
	}
	if requestOptionArgs != nil {
		httpGetArgs = append(httpGetArgs, requestOptionArgs...)
	}

	resp, err := HttpGet(ctx, u, httpGetArgs...)
	if err != nil {
		return nil, fmt.Errorf("http network error: %w", err)
	}
	defer resp.wrapped.Body.Close()

	if resp.StatusCode(ctx) >= 400 {
		return nil, fmt.Errorf("http: status code %d: %s", resp.StatusCode(ctx), resp.Status(ctx))
	}

	b, err = io.ReadAll(resp.wrapped.Body)
	if err != nil {
		return nil, fmt.Errorf("http: error while reading body: %w", err)
	}

	respContentType, err := Mime_(ctx, core.Str(resp.ContentType(ctx)))
	if err == nil && contentType == "" {
		contentType = respContentType
	}

	result, _, finalErr = core.ParseOrValidateResourceContent(ctx, b, contentType, doParse, validateRaw)
	return
}

// httpExists returns true if the argument is a reachable core.Host or a core.URL returning a status code in the range 200-399
func httpExists(ctx *core.Context, args ...core.Value) core.Bool {
	var url core.URL

	for _, arg := range args {
		switch a := arg.(type) {
		case core.Host:
			if url != "" {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("entity"))
			}
			if a.HasScheme() && !a.HasHttpScheme() {
				panic(errors.New("only http(s) hosts are accepted"))
			} else {
				url = core.URL(a + "/")
			}
		case core.URL:
			if url != "" {
				panic(commonfmt.FmtErrArgumentProvidedAtLeastTwice("entity"))
			}
			url = a
		default:
			panic(errors.New("core.URL or core.Host argument expected"))
		}
	}

	if url.UnderlyingString() == "" {
		panic(errors.New("missing argument"))
	}

	client, opts, err := getClientAndOptions(ctx, url)
	if err != nil {
		panic(err)
	}

	req, err := client.MakeRequest(ctx, "HEAD", url, nil, "", opts)
	if err != nil {
		panic(err)
	}
	resp, err := client.DoRequest(ctx, req)
	if err == nil {
		defer resp.wrapped.Body.Close()
	}

	return err == nil && resp.wrapped.StatusCode <= 399
}
