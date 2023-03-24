package internal

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"

	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	help "github.com/inox-project/inox/internal/globals/help"
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

	help.RegisterHelps([]help.TopicHelp{
		{
			Value: _hashPassword,
			Topic: "hash_password",
			Text: "the hash_password function hashes a password string using the Bcrypt hashing algorithm, it accepts the cost " +
				"as a second optional argument. The cost is an integer between 4 and 31, it defaults to 7.",
			Examples: []help.Example{
				{
					Code:   `hash_password("password")`,
					Output: `"JDJhJDA3JHNNNzRwaFVNUFVCNzVDQmxINU5HaWVOZERKZ09IRkx4a2xxYTFPTktsV3Nkd2JqampmYmxT"`,
				},
				{
					Code:   `hash_password("password", 10)`,
					Output: `"JDJhJDEwJGhLODFiVThNdTlJZXVRMXVZdHlIUi5oOS5GSXljNWpYWGcwaVhXWUZYZC5YRTduR1hmSjl1"`,
				},
			},
		},
		{
			Value: _checkPassword,
			Topic: "check_password",
			Text:  "the check_password hashes a string or a byte sequences using the SHA-256 algorithm",
			Examples: []help.Example{
				{
					Code:   `check_password("password", "JDJhJDA3JHNNNzRwaFVNUFVCNzVDQmxINU5HaWVOZERKZ09IRkx4a2xxYTFPTktsV3Nkd2JqampmYmxT")`,
					Output: `true`,
				},
			},
		},
		{
			Value: _sha256,
			Topic: "sha256",
			Text:  "the sha256 function hashes a string or a byte sequence with the SHA-256 algorithm ",
			Examples: []help.Example{
				{
					Code:   `sha256("string")`,
					Output: `0x[473287f8298dba7163a897908958f7c0eae733e25d2e027992ea2edc9bed2fa8]`,
				},
			},
		},
		{
			Value: _sha384,
			Topic: "sha384",
			Text:  "the sha384 function hashes a string or a byte sequence with the SHA-384 algorithm ",
			Examples: []help.Example{
				{
					Code:   `sha384("string")`,
					Output: `0x[36396a7e4de3fa1c2156ad291350adf507d11a8f8be8b124a028c5db40785803ca35a7fc97a6748d85b253babab7953e]`,
				},
			},
		},
		{
			Value: _sha512,
			Topic: "sha512",
			Text:  "the sha512 function hashes a string or a byte sequence with the SHA-512 algorithm ",
			Examples: []help.Example{
				{
					Code:   `sha512("string")`,
					Output: `0x[2757cb3cafc39af451abb2697be79b4ab61d63d74d85b0418629de8c26811b529f3f3780d0150063ff55a2beee74c4ec102a2a2731a1f1f7f10d473ad18a6a87]`,
				},
			},
		},
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
