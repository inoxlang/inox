package internal

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	help "github.com/inoxlang/inox/internal/globals/help"
	"golang.org/x/crypto/bcrypt"
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
	})

	help.RegisterHelpValues(map[string]any{
		"hash_password":  _hashPassword,
		"check_password": _checkPassword,
		"sha256":         _sha256,
		"sha384":         _sha384,
		"sha512":         _sha512,
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

// hash hashes the bytes read from readable using the speficied hashing algorithm
func hash(readable core.Readable, algorithm HashingAlgorithm) []byte {
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
	cost := bcrypt.MinCost + 3

	if len(args) > 1 {
		panic(errors.New("at most one option expected (the cost)"))
	}

	for _, arg := range args {
		if i, ok := arg.(core.Int); ok {
			cost = int(i)
		} else {
			panic(fmt.Errorf("invalid argument %#v, a cost was expected", arg))
		}
	}

	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		panic(fmt.Errorf("failed to hash password: %w", err))
	}
	//TODO: use checked string
	return core.Str(base64.StdEncoding.EncodeToString(b))
}

func _checkPassword(ctx *core.Context, password core.Str, hashed core.Str) core.Bool {
	b, err := base64.StdEncoding.DecodeString(string(hashed))
	if err != nil {
		panic(fmt.Errorf("failed to decode hashed password: %w", err))
	}
	err = bcrypt.CompareHashAndPassword(b, []byte(password))
	return err == nil
}

func _sha256(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: hash(arg, SHA256), IsDataMutable: true}
}

func _sha384(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: hash(arg, SHA384), IsDataMutable: true}
}

func _sha512(_ *core.Context, arg core.Readable) *core.ByteSlice {
	return &core.ByteSlice{Bytes: hash(arg, SHA512), IsDataMutable: true}
}
