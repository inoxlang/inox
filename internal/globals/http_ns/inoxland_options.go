package http_ns

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	core "github.com/inoxlang/inox/internal/core"
)

func getClientAndOptions(ctx *core.Context, u core.URL, requestOptionArgs ...core.Value) (*HttpClient, *HttpRequestOptions, error) {
	options := *DEFAULT_HTTP_REQUEST_OPTIONS
	specifiedOptionNames := make(map[string]int, 0)
	var client *HttpClient

	for _, v := range requestOptionArgs {

		switch optVal := v.(type) {
		case core.QuantityRange:
			if options.Timeout != DEFAULT_HTTP_CLIENT_TIMEOUT {
				return nil, nil, errors.New("http option object: timeout provided already at least twice")
			}
			if d, ok := optVal.InclusiveEnd().(core.Duration); ok {
				options.Timeout = time.Duration(d)
				specifiedOptionNames["timeout"] = 1
			} else {
				return nil, nil, fmt.Errorf("invalid http option: a quantity range with end of type %T", optVal.InclusiveEnd())
			}
		case core.Option:
			switch optVal.Name {
			case "insecure":
				if b, ok := optVal.Value.(core.Bool); ok {
					if b {
						options.InsecureSkipVerify = true
					}
				} else {
					return nil, nil, fmt.Errorf("invalid http option: --insecure should have a boolean value")
				}
			case "client":
				if c, ok := optVal.Value.(*HttpClient); ok {
					client = c
				} else {
					return nil, nil, fmt.Errorf("invalid http option: --client should be an http client")
				}
			default:
				return nil, nil, fmt.Errorf("invalid http option: an option named --%s", optVal.Name)
			}
		default:
			return nil, nil, fmt.Errorf("invalid http option: %#v", optVal)
		}
	}

	if client != nil {
		if specifiedOptionNames["timeout"] == 0 {
			options.Timeout = client.options.Timeout
		}
		//specified options cannot override the profile's jar
		options.Jar = client.options.Jar
	}

	if client == nil {
		c, err := ctx.GetProtolClient(u)
		if err == nil {
			client = c.(*HttpClient)
		} else {
			client = &HttpClient{
				client: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: options.InsecureSkipVerify,
						},
					},
					Timeout: options.Timeout,
					Jar:     options.Jar,
				},
			}
		}
	}

	return client, &options, nil
}
