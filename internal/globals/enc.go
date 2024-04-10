package globals

import (
	"encoding/base64"
	"encoding/hex"
	"io"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

// encodeBase64 encodes to base 64 the bytes read from a readable.
func encodeBase64(_ *core.Context, readable core.Readable) core.String {
	reader := readable.Reader()

	var src []byte
	if reader.AlreadyHasAllData() {
		src = reader.GetBytesDataToNotModify()
	} else {
		slice, err := reader.ReadAll()
		if err != nil {
			panic(err)
		}
		src = slice.UnderlyingBytes()
	}

	return core.String(base64.StdEncoding.EncodeToString(src))
}

// decodeBase64 decodes base64 data read from a readable.
func decodeBase64(_ *core.Context, readable core.Readable) (*core.ByteSlice, error) {
	var encoding = base64.StdEncoding
	reader := readable.Reader()

	if reader.AlreadyHasAllData() {
		src := reader.GetBytesDataToNotModify()
		buf := make([]byte, encoding.DecodedLen(len(src)))

		n, err := encoding.Decode(buf, src)
		return core.NewMutableByteSlice(buf[:n], ""), err
	} else {
		decoder := base64.NewDecoder(encoding, reader)
		b, err := io.ReadAll(decoder)
		return core.NewMutableByteSlice(b, ""), err
	}

}

// encodeHex encodes to hexadecimal the bytes read from a readable.
func encodeHex(_ *core.Context, readable core.Readable) core.String {
	reader := readable.Reader()

	var src []byte
	if reader.AlreadyHasAllData() {
		src = reader.GetBytesDataToNotModify()
	} else {
		slice, err := reader.ReadAll()
		if err != nil {
			panic(err)
		}
		src = slice.UnderlyingBytes()
	}

	return core.String(hex.EncodeToString(src))
}

// decodeHex decodes hex data read from a readable.
func decodeHex(_ *core.Context, readable core.Readable) (*core.ByteSlice, error) {
	reader := readable.Reader()

	if reader.AlreadyHasAllData() {
		src := reader.GetBytesDataToNotModify()

		b, err := hex.DecodeString(utils.BytesAsString(src))
		return core.NewMutableByteSlice(b, ""), err
	} else {
		decoder := hex.NewDecoder(reader)
		b, err := io.ReadAll(decoder)
		return core.NewMutableByteSlice(b, ""), err
	}
}
