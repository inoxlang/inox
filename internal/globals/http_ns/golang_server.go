package http_ns

import (
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	fsutil "github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION = time.Hour * 24 * 180
	RELATIVE_SELF_SIGNED_CERT_FILEPATH         = "./" + inoxconsts.DEV_DIR_NAME + "/self_signed.cert"
	RELATIVE_SELF_SIGNED_CERT_KEY_FILEPATH     = "./" + inoxconsts.DEV_DIR_NAME + "/self_signed.key"
)

type GolangHttpServerConfig struct {
	//hostname:port or :port
	Addr    string
	Handler http.Handler

	PemEncodedCert string
	PemEncodedKey  string

	AllowSelfSignedCertCreationEvenIfExposed bool
	//if true the certificate and key files are persisted on the filesystem for later reuse.
	PersistCreatedLocalCert        bool
	SelfSignedCertValidityDuration time.Duration //defaults to DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION

	ReadHeaderTimeout time.Duration // defaults to DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT
	ReadTimeout       time.Duration // defaults to DEFAULT_HTTP_SERVER_READ_TIMEOUT
	WriteTimeout      time.Duration // defaults to DEFAULT_HTTP_SERVER_WRITE_TIMEOUT
	MaxHeaderBytes    int           // defaults to DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES
}

func NewGolangHttpServer(ctx *core.Context, config GolangHttpServerConfig) (*http.Server, error) {
	fls := ctx.GetFileSystem()

	pemEncodedCert := config.PemEncodedCert
	pemEncodedKey := config.PemEncodedKey

	//if no certificate is provided by the user we create one
	if config.PemEncodedCert == "" && (isLocalhostOr127001Addr(config.Addr) || (isBindAllAddress(config.Addr) && config.AllowSelfSignedCertCreationEvenIfExposed)) {
		initialWorkingDir := ctx.InitialWorkingDirectory()
		var (
			CERT_FILEPATH     = initialWorkingDir.Join(RELATIVE_SELF_SIGNED_CERT_FILEPATH, fls).UnderlyingString()
			CERT_KEY_FILEPATH = initialWorkingDir.Join(RELATIVE_SELF_SIGNED_CERT_KEY_FILEPATH, fls).UnderlyingString()
		)

		validityDuration := utils.DefaultIfZero(config.SelfSignedCertValidityDuration, DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION)
		generateCert := false

		if config.PersistCreatedLocalCert {

			certFileStat, err1 := fls.Stat(CERT_FILEPATH)
			_, err2 := fls.Stat(CERT_KEY_FILEPATH)

			if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) || time.Since(certFileStat.ModTime()) >= validityDuration {
				//generate a new certificate if at least one of the file does not exist or the certificate is no longer valid.
				generateCert = true

				if err1 == nil {
					fls.Remove(CERT_FILEPATH)
				}

				if err2 == nil {
					fls.Remove(CERT_KEY_FILEPATH)
				}
			} else if err1 == nil && err2 == nil {
				//reuse

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
		} else {
			generateCert = true
		}

		if generateCert {
			cert, key, err := GenerateSelfSignedCertAndKey(SelfSignedCertParams{
				Localhost:        true,
				NonLocalhostIPs:  config.AllowSelfSignedCertCreationEvenIfExposed,
				ValidityDuration: DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION,
			})
			if err != nil {
				return nil, err
			}

			pemEncodedCert = string(pem.EncodeToMemory(cert))
			pemEncodedKey = string(pem.EncodeToMemory(key))

			if config.PersistCreatedLocalCert {
				dir := filepath.Dir(CERT_FILEPATH)
				err := fls.MkdirAll(dir, 0700)
				if err != nil {
					goto ignore_writes
				}

				certFile, err := fls.Create(CERT_FILEPATH)
				if err != nil {
					//landlock
					goto ignore_writes
				}
				pem.Encode(certFile, cert)
				certFile.Close()

				keyFile, err := fls.Create(CERT_KEY_FILEPATH)
				if err != nil {
					goto ignore_writes
				}
				pem.Encode(keyFile, key)
				keyFile.Close()
			}
		ignore_writes:
		}
	}

	if pemEncodedCert == "" {
		return nil, errors.New("no certificate was provided and no self-signed certificate was generated")
	}

	tlsConfig, err := GetTLSConfig(ctx, pemEncodedCert, pemEncodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS config: %w", err)
	}

	server := &http.Server{
		Addr:              config.Addr,
		Handler:           config.Handler,
		ReadHeaderTimeout: utils.DefaultIfZero(config.ReadHeaderTimeout, DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT),
		ReadTimeout:       utils.DefaultIfZero(config.ReadTimeout, DEFAULT_HTTP_SERVER_READ_TIMEOUT),
		WriteTimeout:      utils.DefaultIfZero(config.WriteTimeout, DEFAULT_HTTP_SERVER_WRITE_TIMEOUT),
		MaxHeaderBytes:    utils.DefaultIfZero(config.MaxHeaderBytes, DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES),
		TLSConfig:         tlsConfig,
		//TODO: set logger
	}

	return server, nil
}

func isLocalhostOr127001Addr[S ~string](addr S) bool {
	if addr == "localhost" || addr == "127.0.0.1" {
		return true
	}
	return strings.HasPrefix(string(addr), "localhost:") || strings.HasPrefix(string(addr), "127.0.0.1:")
}

func isBindAllAddress[S ~string](addr S) bool {
	if addr == "" {
		return false
	}

	if addr == "0.0.0.0" || addr[0] == ':' {
		return true
	}
	return strings.HasPrefix(string(addr), "0.0.0.0:")
}
