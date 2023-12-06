package compressarch

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	//go:embed simple.tar.gz
	SIMPLE_GZIP_ARCHIVE_BYTES []byte
)

func TestUnGzip(t *testing.T) {

	buf := bytes.NewBuffer(nil)
	err := UnGzip(buf, bytes.NewReader(SIMPLE_GZIP_ARCHIVE_BYTES))
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, SIMPLE_TARBALL_BYTES, buf.Bytes())
}
