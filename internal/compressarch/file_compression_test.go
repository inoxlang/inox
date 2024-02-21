package compressarch

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileCompressor(t *testing.T) {

	t.Run("base case", func(t *testing.T) {
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

	t.Run("parallel", func(t *testing.T) {

		ctx := context.Background()
		compressor := NewFileCompressor()

		updateCount := 10_000
		content := bytes.Repeat([]byte("1"), 100)

		//Create a goroutine that will compress a large amount of files (single time per file).
		go func() {
			for i := 0; i < updateCount; i++ {
				compressor.CompressFileContent(ContentCompressionParams{
					Ctx:           ctx,
					ContentReader: bytes.NewReader(content),
					Path:          "/file" + strconv.Itoa(i) + ".js",
					LastMtime:     time.Now(),
				})
			}
		}()

		//Compress a large amount of files (single time per file).
		for i := 0; i < updateCount; i++ {

			reader, isCompressed, err := compressor.CompressFileContent(ContentCompressionParams{
				Ctx:           ctx,
				ContentReader: bytes.NewReader(content),
				Path:          "/file" + strconv.Itoa(i) + ".css",
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
			assert.Equal(t, content, buf.Bytes())
		}

	})
}
