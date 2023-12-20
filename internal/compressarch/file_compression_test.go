package compressarch

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileCompressor(t *testing.T) {

	t.Run("", func(t *testing.T) {
		ctx := context.Background()
		compressor := NewFileCompressor()
		fpath := "/index.js"

		initialContent := bytes.Repeat([]byte("1"), 100)

		reader, isCompressed, err := compressor.CompressFileContent(ContentCompressionParams{
			Ctx:           ctx,
			ContentReader: bytes.NewReader(initialContent),
			Path:          fpath,
			LastMtime:     time.Now(),
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, isCompressed) {
			return
		}

		buf := bytes.NewBuffer(nil)
		err = UnGzip(buf, reader)

		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, initialContent, buf.Bytes())

		// compress the new content

		secondContent := bytes.Repeat([]byte("2"), 50) //smaller on purpose.

		firstUpdate := time.Now()

		reader, isCompressed, err = compressor.CompressFileContent(ContentCompressionParams{
			Ctx:           ctx,
			ContentReader: bytes.NewReader(secondContent),
			Path:          fpath,
			LastMtime:     firstUpdate,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, isCompressed) {
			return
		}

		buf = bytes.NewBuffer(nil)
		err = UnGzip(buf, reader)

		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, secondContent, buf.Bytes())

		// if the file has not changes the content reader should not be read.
		contentReader := bytes.NewReader([]byte("xxx"))

		reader, isCompressed, err = compressor.CompressFileContent(ContentCompressionParams{
			Ctx:           ctx,
			ContentReader: contentReader,
			Path:          fpath,
			LastMtime:     firstUpdate,
		})

		if !assert.NoError(t, err) {
			return
		}

		unreadBytes := contentReader.Len()
		assert.EqualValues(t, contentReader.Size(), unreadBytes)

		if !assert.True(t, isCompressed) {
			return
		}

		buf = bytes.NewBuffer(nil)
		err = UnGzip(buf, reader)

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, secondContent, buf.Bytes())
	})

}
