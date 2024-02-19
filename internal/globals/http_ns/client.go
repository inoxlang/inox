package http_ns

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	_cookiejar "net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/context"
	"golang.org/x/net/publicsuffix"

	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
)

var (
	CLIENT_SUPPORTED_SCHEMES = []core.Scheme{"http", "https"}
)

func init() {
	core.CreateHttpClient = func(insecure, saveCookies bool) (core.ProtocolClient, error) {
		return NewClientWithConfig(ClientConfig{
			Insecure:    insecure,
			SaveCookies: saveCookies,
		})
	}
}

var (
	DEFAULT_HTTP_PROFILE_CONFIG = ClientConfig{
		SaveCookies: false,
	}
	_ = []core.ProtocolClient{(*Client)(nil)}
)

// A Client represents a high level http client, Client implements core.ProtocolClient.
type Client struct {
	config  ClientConfig
	options RequestOptions

	client *http.Client
}

func NewClient(ctx *core.Context, configObject *core.Object) (*Client, error) {
	config := ClientConfig{}

	err := configObject.ForEachEntry(func(name string, value core.Serializable) error {
		switch name {
		case "save-cookies":
			saveCookies, ok := value.(core.Bool)
			if !ok {
				return core.FmtPropOfArgXShouldBeOfTypeY("save-cookies", "configuration", "boolean", value)
			}
			config.SaveCookies = bool(saveCookies)
		case "request-finalization":
			finalization, ok := value.(*core.Dictionary)
			if !ok {
				return core.FmtPropOfArgXShouldBeOfTypeY("request-finalization", "configuration", "dictionary", value)
			}
			config.Finalization = finalization
		case "insecure":
			insecure, ok := value.(core.Bool)
			if !ok {
				return core.FmtPropOfArgXShouldBeOfTypeY("insecure", "configuration", "boolean", value)
			}
			config.Insecure = bool(insecure)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return NewClientWithConfig(config)
}

func NewClientWithConfig(config ClientConfig) (*Client, error) {
	client := &Client{
		config:  config,
		options: RequestOptions{},
	}

	if config.SaveCookies {
		jar, err := _cookiejar.New(&_cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return nil, fmt.Errorf("failes to create http client: %w", err)
		}
		client.options.Jar = jar
	}

	client.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.Insecure,
			},
		},
		Jar: client.options.Jar,
	}

	return client, nil
}

func NewClientFromGolangHTTPClient(client *http.Client, insecure bool) *Client {
	return &Client{
		client: client,
		config: ClientConfig{
			Insecure:    insecure,
			SaveCookies: client.Jar != nil,
		},
		options: RequestOptions{
			Timeout:            client.Timeout,
			InsecureSkipVerify: insecure,
			Jar:                client.Jar,
		},
	}
}

func newInsecureGolangHttpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 30 * time.Second,
	}
}

func (c *Client) Schemes() []core.Scheme {
	return CLIENT_SUPPORTED_SCHEMES
}

func (c *Client) IsStateful() bool {
	//TODO: also return true if the client reuse connections.
	return c.config.SaveCookies
}

func (c *Client) MayPurposefullySkipAuthentication() bool {
	return c.config.Insecure
}

func (c *Client) GetHostCookies(h core.Host) []*http.Cookie {
	if c.client.Jar == nil {
		return nil
	}
	u, _ := url.Parse(string(h.HostWithoutPort()))
	return c.client.Jar.Cookies(u)
}

func (c *Client) GetHostCookieObjects(ctx *core.Context, h core.Host) *core.List {
	var objects []core.Serializable
	for _, cookie := range c.GetHostCookies(h) {
		objects = append(objects, createObjectFromCookie(ctx, *cookie))
	}

	return core.NewWrappedValueList(objects...)
}

func (c *Client) MakeRequest(ctx *core.Context, method string, u core.URL, body io.Reader, contentType string, opts *RequestOptions) (*Request, error) {
	perm, err := getPermForRequest(method, u)
	if err != nil {
		return nil, err
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	ctx.Take(HTTP_REQUEST_RATE_LIMIT_NAME, 1*core.FREQ_LIMIT_SCALE)

	req, err := http.NewRequest(method, string(u), body)

	if contentType != "" {
		if utils.SliceContains(spec.METHODS_WITH_NO_BODY, method) {
			req.Header.Add("Accept", string(contentType))
		} else {
			req.Header.Add("Content-Type", string(contentType))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to make request: %s", err.Error())
	}

	if opts.Timeout != 0 {
		newCtx, _ := context.WithTimeout(ctx, opts.Timeout)
		req = req.WithContext(newCtx)
	} else {
		req = req.WithContext(ctx)
	}

	wrapped, err := NewClientSideRequest(req)
	if err != nil {
		return nil, err
	}

	// finalize request
	finalizationConfig := c.config.Finalization
	if finalizationConfig != nil {
		host := wrapped.URL.Host()
		hostConfig, _ := finalizationConfig.Value(ctx, host)
		obj, ok := hostConfig.(*core.Object)
		if ok {
			headers, ok := obj.Prop(ctx, "add-headers").(*core.Object)
			if ok {
				for _, k := range headers.Keys(ctx) {
					v := headers.Prop(ctx, k)

					s, ok := v.(core.String)
					if !ok {
						return nil, errors.New("failed to finalize request: header values should be strings")
					}
					req.Header.Add(k, string(s))
				}
			} else {
				return nil, errors.New("failed to finalize request: .add-headers should be an object")
			}
		}
	}

	// wrap request in an Inox value

	return wrapped, nil
}

func (c *Client) DoRequest(ctx *core.Context, req *Request) (*Response, error) {
	return c.DoStdlibRequest(ctx, req.Request())
}

func (c *Client) DoStdlibRequest(ctx *core.Context, req *http.Request) (*Response, error) {
	ctx.PauseCPUTimeDepletion()
	defer ctx.ResumeCPUTimeDepletion()

	resp, err := c.client.Do(req)
	if resp == nil {
		return nil, err
	}

	return &Response{wrapped: resp}, err
}

type RequestOptions struct {
	Timeout            time.Duration
	InsecureSkipVerify bool
	Jar                http.CookieJar
}

type ClientConfig struct {
	Insecure     bool
	SaveCookies  bool
	Finalization *core.Dictionary
}

func getPermForRequest(method string, u core.URL) (core.HttpPermission, error) {
	method = strings.ToUpper(method)

	var perm core.HttpPermission
	switch method {
	case "GET", "HEAD", "OPTIONS":
		perm = core.HttpPermission{
			Kind_:  permkind.Read,
			Entity: u,
		}
	case "POST", "PATCH":
		perm = core.HttpPermission{
			Kind_:  permkind.Write,
			Entity: u,
		}
	case "DELETE":
		perm = core.HttpPermission{
			Kind_:  permkind.Delete,
			Entity: u,
		}
	default:
		return core.HttpPermission{}, errors.New("following http method is not supported: " + method)
	}

	return perm, nil
}
