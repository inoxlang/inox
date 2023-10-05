package http_ns

import (
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"

	fsutil "github.com/go-git/go-billy/v5/util"
	core "github.com/inoxlang/inox/internal/core"
)

type GolangHttpServerConfig struct {
	Addr           string
	Handler        http.Handler
	PemEncodedCert string
	PemEncodedKey  string
}

func NewGolangHttpServer(ctx *core.Context, config GolangHttpServerConfig) (*http.Server, error) {
	fls := ctx.GetFileSystem()

	pemEncodedCert := config.PemEncodedCert
	pemEncodedKey := config.PemEncodedKey

	if config.PemEncodedCert == "" { //if no certificate provided by the user we create one
		//we generate a self signed certificate that we write to disk so that
		//we can reuse it
		CERT_FILEPATH := "localhost.cert"
		CERT_KEY_FILEPATH := "localhost.key"

		_, err1 := fls.Stat(CERT_FILEPATH)
		_, err2 := fls.Stat(CERT_KEY_FILEPATH)

		if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) {

			if err1 == nil {
				fls.Remove(CERT_FILEPATH)
			}

			if err2 == nil {
				fls.Remove(CERT_KEY_FILEPATH)
			}

			cert, key, err := generateSelfSignedCertAndKey()
			if err != nil {
				return nil, err
			}

			certFile, err := fls.Create(CERT_FILEPATH)
			if err != nil {
				return nil, err
			}
			pem.Encode(certFile, cert)
			pemEncodedCert = string(pem.EncodeToMemory(cert))

			certFile.Close()
			keyFile, err := fls.Create(CERT_KEY_FILEPATH)
			if err != nil {
				return nil, err
			}
			pem.Encode(keyFile, key)
			keyFile.Close()
			pemEncodedKey = string(pem.EncodeToMemory(key))
		} else if err1 == nil && err2 == nil {
			certFile, err := fsutil.ReadFile(fls, CERT_FILEPATH)
			if err != nil {
				return nil, err
			}
			keyFile, err := fsutil.ReadFile(fls, CERT_KEY_FILEPATH)
			if err != nil {
				return nil, err
			}

			pemEncodedCert = string(certFile)
			pemEncodedKey = string(keyFile)
		} else {
			return nil, fmt.Errorf("%w %w", err1, err2)
		}
	}

	tlsConfig, err := GetTLSConfig(ctx, pemEncodedCert, pemEncodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS config: %w", err)
	}

	server := &http.Server{
		Addr:              config.Addr,
		Handler:           config.Handler,
		ReadHeaderTimeout: DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT,
		ReadTimeout:       DEFAULT_HTTP_SERVER_READ_TIMEOUT,
		WriteTimeout:      DEFAULT_HTTP_SERVER_WRITE_TIMEOUT,
		MaxHeaderBytes:    DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES,
		TLSConfig:         tlsConfig,
		//TODO: set logger
	}

	return server, nil
}
