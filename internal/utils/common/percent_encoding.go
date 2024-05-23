package utils

import (
	"errors"
)

var (
	ErrInvalidPercentEncodedString = errors.New("invalid percent encoded string")
)

func PercentEncode(s string) string {
	encoded := make([]byte, 3*len(s))

	for i, b := range StringAsBytes(s) {
		base := i * 3
		encoded[base] = '%'
		encoded[base+1] = UPPER_HEX_DIGITS[b>>4]
		encoded[base+2] = UPPER_HEX_DIGITS[b&15]
	}

	return BytesAsString(encoded)
}

func PercentDecode(s string, allowNotEncoded bool) (string, error) {
	var decoded []byte
	if allowNotEncoded {
		decoded = make([]byte, 0, len(s)) //minimum capacity
	} else {
		if (len(s) % 3) != 0 {
			return "", ErrInvalidPercentEncodedString
		}

		decoded = make([]byte, 0, len(s)/3) //exact capacity
		decoded = decoded[0:len(decoded):len(decoded)]
	}

	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && IsHexDigit(s[i+1]) && IsHexDigit(s[i+2]) {
			decoded = append(decoded, (HexDigitToByte(s[i+1])<<4 | HexDigitToByte(s[i+2])))
			i += 2
		} else { // if not valid
			if allowNotEncoded && s[i] != '%' {
				decoded = append(decoded, s[i])
				continue
			}
			return "", ErrInvalidPercentEncodedString
		}
	}
	return BytesAsString(decoded), nil
}
