package internal

import (
	"encoding/base64"
	"io/ioutil"

	core "github.com/inox-project/inox/internal/core"
)

// encodeBase64 encodes to base 64 the bytes read from a readable.
func encodeBase64(_ *core.Context, readable core.Readable) core.Str {
	reader := readable.Reader()

	var src []byte
	if reader.AlreadyHasAllData() {
		src = reader.GetBytesDataToNotModify()
	} else {
		slice, err := reader.ReadAll()
		if err != nil {
			panic(err)
		}
		src = slice.Bytes
	}

	return core.Str(base64.StdEncoding.EncodeToString(src))
}

// decodeBase64 decodes base64 data read from a readable.
func decodeBase64(_ *core.Context, readable core.Readable) (*core.ByteSlice, error) {
	var encoding = base64.StdEncoding
	reader := readable.Reader()

	if reader.AlreadyHasAllData() {
		src := reader.GetBytesDataToNotModify()
		buf := make([]byte, encoding.DecodedLen(len(src)))

		n, err := encoding.Decode(buf, src)
		return &core.ByteSlice{Bytes: buf[:n], IsDataMutable: true}, err
	} else {
		decoder := base64.NewDecoder(encoding, reader)
		b, err := ioutil.ReadAll(decoder)
		return &core.ByteSlice{Bytes: b, IsDataMutable: true}, err
	}

}
