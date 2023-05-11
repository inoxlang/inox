package internal

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"mime"
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

func Mime_(ctx *core.Context, arg core.Str) (core.Mimetype, error) {
	switch arg {
	case "json":
		arg = core.JSON_CTYPE
	case "yaml":
		arg = core.APP_YAML_CTYPE
	case "text":
		arg = core.PLAIN_TEXT_CTYPE
	}

	_, _, err := mime.ParseMediaType(string(arg))
	if err != nil {
		return "", fmt.Errorf("'%s' is not a MIME type: %s", arg, err.Error())
	}

	return core.Mimetype(arg), nil
}
