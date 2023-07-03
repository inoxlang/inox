package internal

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/alexedwards/argon2id"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/help_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_RSA_KEY_SIZE    = 2048 //bit count
	ARGON2ID_HASH_SALT_SEP  = "|"
	MAX_PASSWORD_BYTE_COUNT = 100
)

var (
	PEM_PRIVATE_KEY_PATTERN   = core.NewSecretPattern(core.NewPEMRegexPattern("(RSA )?PRIVATE KEY"), true)
	KEY_PAIR_RECORD_PROPNAMES = []string{"public", "private"}

	SYMB_KEY_PAIR_RECORD = symbolic.NewRecord(map[string]symbolic.SymbolicValue{
		"public":  symbolic.ANY_STR,
		"private": symbolic.ANY_SECRET,
	})

	DEFAULT_ARGON2ID_PARAMS = argon2id.Params{
		Memory:      argon2id.DefaultParams.Memory,
		Iterations:  argon2id.DefaultParams.Iterations,
		Parallelism: 1,
		SaltLength:  argon2id.DefaultParams.SaltLength,
		KeyLength:   argon2id.DefaultParams.KeyLength,
	}

	ErrPasswordTooLong = errors.New("password is too long")
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		_hashPassword, func(ctx *symbolic.Context, arg *symbolic.String, args ...symbolic.SymbolicValue) *symbolic.String {
			return &symbolic.String{}
		},
		_checkPassword, func(ctx *symbolic.Context, pass *symbolic.String, hash *symbolic.String) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		_sha256, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},

		_sha384, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_sha512, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_rsa_gen_key, func(ctx *symbolic.Context) *symbolic.Record {
			return SYMB_KEY_PAIR_RECORD
		},
		_rsa_encrypt_oaep, func(ctx *symbolic.Context, readable symbolic.Readable, pubKey symbolic.StringLike) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},
		_rsa_decrypt_oaep, func(ctx *symbolic.Context, readable symbolic.Readable, key *symbolic.Secret) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},
	})

	help_ns.RegisterHelpValues(map[string]any{
		"hash_password":  _hashPassword,
		"check_password": _checkPassword,
		"sha256":         _sha256,
		"sha384":         _sha384,
		"sha512":         _sha512,

		"rsa.gen_key":      _rsa_gen_key,
		"rsa.encrypt_oaep": _rsa_encrypt_oaep,
		"rsa.decrypt_oaep": _rsa_decrypt_oaep,
	})
}

type HashingAlgorithm int

const (
	SHA256 HashingAlgorithm = iota
	SHA384
	SHA512
	SHA1
	MD5
)

func (alg HashingAlgorithm) String() string {
	switch alg {
	case SHA256:
		return "SHA256"
	case SHA384:
		return "SHA384"
	case SHA512:
		return "SHA512"
	case SHA1:
		return "SHA1"
	case MD5:
		return "MD5"
	default:
		panic(errors.New("unknown hashing algorithm"))
	}
}

// _hash hashes the bytes read from readable using the speficied hashing algorithm
func _hash(readable core.Readable, algorithm HashingAlgorithm) []byte {
	reader := readable.Reader()

	//TODO: create hash for large inputs

	var b []byte

	if reader.AlreadyHasAllData() {
		b = reader.GetBytesDataToNotModify()
	} else {
		slice, err := reader.ReadAll()
		if err != nil {
			panic(err)
		}
		b = slice.Bytes
	}

	switch algorithm {
	case SHA256:
		arr := sha256.Sum256(b)
		return arr[:]
	case SHA384:
		arr := sha512.Sum384(b)
		return arr[:]
	case SHA512:
		arr := sha512.Sum512(b)
		return arr[:]
	case MD5:
		arr := md5.Sum(b)
		return arr[:]
	case SHA1:
		arr := sha1.Sum(b)
		return arr[:]
	default:
		panic(errors.New("invalid hashing algorithm"))
	}
}

func _hashPassword(ctx *core.Context, password core.Str, args ...core.Value) core.Str {
	if len(string(password)) > MAX_PASSWORD_BYTE_COUNT {
		panic(ErrPasswordTooLong)
	}

	hash := utils.Must(argon2id.CreateHash(string(password), &DEFAULT_ARGON2ID_PARAMS))
	return core.Str(hash)
}

func _checkPassword(ctx *core.Context, password core.Str, hash core.Str) core.Bool {
	if len(string(password)) > MAX_PASSWORD_BYTE_COUNT {
		panic(ErrPasswordTooLong)
	}

	ok := utils.Must(argon2id.ComparePasswordAndHash(string(password), string(hash)))
	return core.Bool(ok)
}

func _sha256(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: _hash(arg, SHA256), IsDataMutable: true}
}

func _sha384(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: _hash(arg, SHA384), IsDataMutable: true}
}

func _sha512(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: _hash(arg, SHA512), IsDataMutable: true}
}

func _rsa_gen_key(ctx *core.Context) *core.Record {
	privateKey, _ := rsa.GenerateKey(rand.Reader, DEFAULT_RSA_KEY_SIZE)
	publicKey := &privateKey.PublicKey

	privKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privKeyPem := string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	}))

	pubKeyBytes := utils.Must(x509.MarshalPKIXPublicKey(publicKey))
	pubKeyPem := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}))

	return core.NewRecordFromKeyValLists(KEY_PAIR_RECORD_PROPNAMES, []core.Value{
		core.Str(pubKeyPem), utils.Must(PEM_PRIVATE_KEY_PATTERN.NewSecret(ctx, privKeyPem)),
	})
}

func _rsa_encrypt_oaep(_ *core.Context, arg core.Readable, key core.StringLike) (*core.ByteSlice, error) {
	pubKeyPEM, err := decodeAlonePEM(key.GetOrBuildString())
	if err != nil {
		return nil, fmt.Errorf("failed to decode PEM: %w", err)
	}
	_pubKey, err := x509.ParsePKIXPublicKey(pubKeyPEM.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKIX public key: %w", err)
	}

	pubKey := _pubKey.(*rsa.PublicKey)

	slice, err := arg.Reader().ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read all data to encrypt: %w", err)
	}

	bytes := utils.CopySlice(slice.Bytes)

	encrypted, err := rsa.EncryptOAEP(sha256.New(), core.CryptoRandSource, pubKey, bytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	return core.NewByteSlice(encrypted, false, ""), nil
}

func _rsa_decrypt_oaep(_ *core.Context, arg core.Readable, key *core.Secret) (*core.ByteSlice, error) {
	key.AssertIsPattern(PEM_PRIVATE_KEY_PATTERN)

	privKeyPEM, err := key.DecodedPEM()
	if err != nil {
		return nil, fmt.Errorf("failed to decode PEM: %w", err)
	}

	privKey, err := x509.ParsePKCS1PrivateKey(privKeyPEM.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS1 private key: %w", err)
	}

	slice, err := arg.Reader().ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read all data to decrypt: %w", err)
	}

	bytes := utils.CopySlice(slice.Bytes)

	decrypted, err := rsa.DecryptOAEP(sha256.New(), core.CryptoRandSource, privKey, bytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return core.NewByteSlice(decrypted, false, ""), nil
}

func newRSANamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"encrypt_oaep": core.WrapGoFunction(_rsa_encrypt_oaep),
		"decrypt_oaep": core.WrapGoFunction(_rsa_decrypt_oaep),
		"gen_key":      core.WrapGoFunction(_rsa_gen_key),
	})
}

func decodeAlonePEM(s string) (*pem.Block, error) {
	block, rest := pem.Decode(utils.StringAsBytes(s))
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("PEM encoded secret is followed by non space charaters")
	}

	return block, nil
}
