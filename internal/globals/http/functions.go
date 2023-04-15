package internal

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "github.com/inoxlang/inox/internal/core"
)

const (
	HTTP_UPLOAD_RATE_LIMIT_NAME  = "http/upload"
	HTTP_REQUEST_RATE_LIMIT_NAME = "http/request"

	DEFAULT_HTTP_CLIENT_TIMEOUT = 10 * time.Second

	MISSING_URL_ARG           = "missing core.URL argument"
	OPTION_DOES_NOT_EXIST_FMT = "option '%s' does not exist"
)

var DEFAULT_HTTP_REQUEST_OPTIONS = &HttpRequestOptions{
	Timeout:            DEFAULT_HTTP_CLIENT_TIMEOUT,
	InsecureSkipVerify: false,
}

func getPublicKey(privKey interface{}) interface{} {
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func makePemBlockForKey(privKey interface{}) (*pem.Block, error) {
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(k),
		}, nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal ECDSA private key: %v", err)

		}
		return &pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: b,
		}, nil
	default:
		return nil, fmt.Errorf("cannot make PEM block for %#v", k)
	}
}

func generateSelfSignedCertAndKey() (cert *pem.Block, key *pem.Block, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.DNSNames = append(template.DNSNames, "localhost")

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, getPublicKey(priv), priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %s", err)

	}

	keyBlock, err := makePemBlockForKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create key: %s", err)
	}

	return &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}, keyBlock, nil
}

func getClientAndOptions(ctx *core.Context, u core.URL, obj *core.Object) (*HttpClient, *HttpRequestOptions, error) {
	options := *DEFAULT_HTTP_REQUEST_OPTIONS
	specifiedOptionNames := make(map[string]int, 0)
	var client *HttpClient

	for k, v := range obj.EntryMap() {

		//CHECK KEY

		_, err := strconv.ParseUint(k, 10, 32)
		if err != nil {
			return nil, nil, errors.New("http option object: only integer keys are supported for now")
		}

		//CHECK VALUE

		switch optVal := v.(type) {
		case core.QuantityRange:
			if options.Timeout != DEFAULT_HTTP_CLIENT_TIMEOUT {
				return nil, nil, errors.New("http option object: timeout provided already at least twice")
			}
			if d, ok := optVal.End.(core.Duration); ok {
				options.Timeout = time.Duration(d)
				specifiedOptionNames["timeout"] = 1
			} else {
				return nil, nil, fmt.Errorf("invalid http option: a quantity range with end of type %T", optVal.End)
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

func makeHttpClient(opts *HttpRequestOptions) *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.InsecureSkipVerify,
			},
		},
		Timeout: opts.Timeout,
		Jar:     opts.Jar,
	}

	return client
}

// httpExists returns true if the argument is a reachable core.Host or a core.URL returning a status code in the range 200-399
func httpExists(ctx *core.Context, args ...core.Value) core.Bool {
	var url core.URL

	for _, arg := range args {
		switch a := arg.(type) {
		case core.Host:
			if url != "" {
				panic(core.FmtErrArgumentProvidedAtLeastTwice("entity"))
			}
			if a.HasScheme() && !a.HasHttpScheme() {
				panic(errors.New("only http(s) hosts are accepted"))
			} else {
				url = core.URL(a + "/")
			}
		case core.URL:
			if url != "" {
				panic(core.FmtErrArgumentProvidedAtLeastTwice("entity"))
			}
			url = a
		default:
			panic(errors.New("core.URL or core.Host argument expected"))
		}
	}

	if url.UnderlyingString() == "" {
		panic(errors.New("missing argument"))
	}

	client, opts, err := getClientAndOptions(ctx, url, nil)
	if err != nil {
		panic(err)
	}

	req, err := client.MakeRequest(ctx, "HEAD", url, nil, "", opts)
	if err != nil {
		panic(err)
	}
	resp, err := client.DoRequest(ctx, req)
	return err == nil && resp.wrapped.StatusCode <= 399
}

func HttpGet(ctx *core.Context, u core.URL, args ...core.Value) (*HttpResponse, error) {
	var contentType core.Mimetype
	var optionObject *core.Object

	for _, arg := range args {
		switch argVal := arg.(type) {
		case core.URL:
			return nil, core.FmtErrArgumentProvidedAtLeastTwice("url")
		case core.Mimetype:
			if contentType != "" {
				return nil, core.FmtErrXProvidedAtLeastTwice("mime type")
			}
			contentType = argVal
		case *core.Object:
			if optionObject != nil {
				return nil, core.FmtErrXProvidedAtLeastTwice("http option")
			}
			optionObject = argVal
		default:
			return nil, fmt.Errorf("invalid argument, type = %T ", arg)
		}
	}

	//checks

	if u == "" {
		return nil, errors.New(MISSING_URL_ARG)
	}

	client, opts, err := getClientAndOptions(ctx, u, optionObject)
	if err != nil {
		return nil, err
	}

	req, err := client.MakeRequest(ctx, "GET", u, nil, string(contentType), opts)
	if err != nil {
		return nil, err
	}
	return client.DoRequest(ctx, req)
}

func httpGetBody(ctx *core.Context, u core.URL, args ...core.Value) (*core.ByteSlice, error) {
	resp, err := HttpGet(ctx, u, args...)
	if err != nil {
		return &core.ByteSlice{}, err
	}
	b, err := io.ReadAll(resp.wrapped.Body)
	if err != nil {
		return &core.ByteSlice{}, err
	}
	return &core.ByteSlice{Bytes: b, IsDataMutable: true}, nil
}

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
	var optionObject *core.Object

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

				if optionObject != nil {
					return nil, core.FmtErrXProvidedAtLeastTwice("http option")
				}

				optionObject = argVal
			}
		default:
			return nil, fmt.Errorf("only an URL argument is expected, not a(n) %T ", arg)
		}
	}

	//checks

	if u == "" {
		return nil, errors.New(MISSING_URL_ARG)
	}

	client, opts, err := getClientAndOptions(ctx, u, optionObject)
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
	var optionObject *core.Object

	for _, arg := range args {
		switch argVal := arg.(type) {
		case core.URL:
			if u != "" {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice("url")
			}
			u = argVal
		case *core.Object:
			if optionObject != nil {
				return nil, core.FmtErrXProvidedAtLeastTwice("http option")
			}
			optionObject = argVal
		default:
			return nil, fmt.Errorf("only an core.URL argument is expected, not a(n) %T ", arg)
		}
	}

	//checks

	if u == "" {
		return nil, errors.New(MISSING_URL_ARG)
	}

	client, opts, err := getClientAndOptions(ctx, u, optionObject)
	if err != nil {
		return nil, err
	}

	req, err := client.MakeRequest(ctx, "DELETE", u, nil, "", opts)
	if err != nil {
		return nil, err
	}
	return client.DoRequest(ctx, req)
}

func serveFile(ctx *core.Context, rw *HttpResponseWriter, r *HttpRequest, pth core.Path) error {

	pth = pth.ToAbs()
	perm := core.FilesystemPermission{Kind_: core.ReadPerm, Entity: pth}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	http.ServeFile(rw.rw, r.Request(), string(pth))
	return nil
}

func Mime_(ctx *core.Context, arg core.Str) (core.Mimetype, error) {
	switch arg {
	case "json":
		arg = core.JSON_CTYPE
	case "text":
		arg = core.PLAIN_TEXT_CTYPE
	}

	_, _, err := mime.ParseMediaType(string(arg))
	if err != nil {
		return "", fmt.Errorf("'%s' is not a MIME type: %s", arg, err.Error())
	}

	return core.Mimetype(arg), nil
}
