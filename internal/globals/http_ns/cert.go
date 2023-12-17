package http_ns

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
	"log"
	"math/big"
	"net"
	"strconv"
	"time"

	"github.com/caddyserver/certmagic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	HTTP_UPLOAD_RATE_LIMIT_NAME  = "http/upload"
	HTTP_REQUEST_RATE_LIMIT_NAME = "http/request"

	DEFAULT_HTTP_CLIENT_TIMEOUT = 10 * time.Second

	MISSING_URL_ARG           = "missing core.URL argument"
	OPTION_DOES_NOT_EXIST_FMT = "option '%s' does not exist"

	CERT_MAGIG_LOG_SRC = "http/certmagic"
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

type SelfSignedCertParams struct {
	Localhost        bool
	NonLocalhostIPs  bool
	ValidityDuration time.Duration //should be >= 100ms, defaults to DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION
}

func GenerateSelfSignedCertAndKey(args SelfSignedCertParams) (cert *pem.Block, key *pem.Block, err error) {
	if !args.Localhost && !args.NonLocalhostIPs {
		return nil, nil, errors.New("invalid arguments: both .Localhost and .NonLocalhostIPs are false")
	}

	if args.ValidityDuration == 0 {
		args.ValidityDuration = DEFAULT_SELF_SIGNED_CERT_VALIDITY_DURATION
	} else if args.ValidityDuration < 100*time.Millisecond {
		return nil, nil, errors.New("invalid arguments: validity duration is less than 100ms")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	//Generate a random organization name. Re-using the same (org. name, serial number) pair
	//can cause an error (SEC_ERROR_REUSED_ISSUER_AND_SERIAL) in browsers such as Firefox.
	orgName := "Acme Co - " + strconv.FormatUint(core.DefaultRandSource.Uint64(), 32)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{orgName},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(args.ValidityDuration),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if args.Localhost {
		template.DNSNames = append(template.DNSNames, "localhost", "127.0.0.1")
	}
	if args.NonLocalhostIPs {
		ips, err := getInterfaceIPs()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list interfaces IPs: %w", err)
		}

		for _, ip := range ips {
			template.DNSNames = append(template.DNSNames, ip.String())
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, getPublicKey(priv), priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	keyBlock, err := makePemBlockForKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create key: %w", err)
	}

	return &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}, keyBlock, nil
}

func generateSelfSignedCertAndKeyValues(ctx *core.Context, args SelfSignedCertParams) (core.Str, *core.Secret, error) {
	cert, key, err := GenerateSelfSignedCertAndKey(args)
	if err != nil {
		return "", nil, err
	}

	secret, err := core.SECRET_PEM_STRING_PATTERN.NewSecret(ctx, string(pem.EncodeToMemory(key)))
	if err != nil {
		return "", nil, err
	}

	return core.Str(pem.EncodeToMemory(cert)), secret, nil
}

func GetTLSConfig(ctx *core.Context, pemEncodedCert string, pemEncodedKey string) (*tls.Config, error) {
	var zapLogger *zap.Logger
	{
		zeroLog := ctx.NewChildLoggerForInternalSource(CERT_MAGIG_LOG_SRC)

		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(utils.FnWriter{
				WriteFn: func(p []byte) (n int, err error) {
					zeroLog.Debug().Msg(utils.BytesAsString(p))
					return len(p), nil
				},
			}),
			zap.DebugLevel,
		)
		zapLogger = zap.New(core)
	}

	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(cert certmagic.Certificate) (*certmagic.Config, error) {

			// Customize the config for the certificate
			return &certmagic.Config{
				Logger: zapLogger,
				//TODO: Storage
			}, nil
		},
		Logger: zapLogger,
		// ...
	})

	magic := certmagic.New(cache, certmagic.Config{
		Logger: zapLogger,
		// Customizations go here
	})

	// myACME := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
	// 	CA:     certmagic.LetsEncryptProductionCA,
	// 	Email:  "you@yours.com",
	// 	Agreed: true,
	// 	// Other customizations
	// })
	// magic.Issuers = []certmagic.Issuer{myACME}

	// err := magic.ManageSync(context.TODO(), []string{"example.com", "sub.example.com"})
	// if err != nil {
	// 	return nil, err
	// }

	cert, err := tls.X509KeyPair([]byte(pemEncodedCert), []byte(pemEncodedKey))
	if err != nil {
		log.Fatalf("failed to create tls.Certificate: %v", err)
	}

	magic.CacheUnmanagedTLSCertificate(ctx, cert, nil)

	tlsConfig := magic.TLSConfig()
	tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

	return tlsConfig, nil
}

// getInterfaceIPs returns the IP addresses of interfaces of type IP network (loopback excluded).
func getInterfaceIPs() ([]net.IP, error) {
	var ips []net.IP
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addresses {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To16() != nil /*IPv4 or IPv6*/ {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	return ips, nil
}
