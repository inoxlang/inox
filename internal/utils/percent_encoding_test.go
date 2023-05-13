package utils

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPercentEncoding(t *testing.T) {

	cases := []string{"1", "12", "a", "ab", "à", "aà", "àa", "😊", "😊a", "a😊"}

	for _, testCase := range cases {
		t.Run(testCase, func(t *testing.T) {
			encoded := PercentEncode(testCase)
			decodedStdlib, err := url.PathUnescape(encoded)

			if !assert.NoError(t, err) {
				return
			}

			if !assert.Equal(t, testCase, decodedStdlib) {
				return
			}

			decoded, err := PercentDecode(encoded)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, testCase, decoded)
		})
	}

}
