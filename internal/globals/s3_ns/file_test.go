package s3_ns

import (
	"math/rand"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestS3WriteFileSeek(t *testing.T) {

	setup := func(ctx *core.Context) (*s3WriteFile, *Bucket) {
		bucket, err := OpenBucketWithCredentials(ctx,
			OpenBucketWithCredentialsInput{
				Provider:   "cloudflare",
				BucketName: "test",
				S3Host:     core.Host("s3://file-test"),
				HttpsHost:  core.Host(S3_FS_TEST_ENDPOINT),
				AccessKey:  S3_FS_TEST_ACCESS_KEY,
				SecretKey:  S3_FS_TEST_SECRET_KEY,
			})

		if !assert.NoError(t, err) {
			t.FailNow()
		}

		fls := NewS3Filesystem(ctx, bucket)

		storageSize := atomic.Int64{}

		file, err := newS3WriteFile(ctx, newS3WriteFileInput{
			client:      bucket.client,
			fs:          fls,
			filename:    "file.txt",
			maxStorage:  1000,
			storageSize: &storageSize,
			flag:        os.O_RDWR | os.O_CREATE,
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return file, bucket
	}

	t.Run("Seek should be thread safe", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.Cancel()

		file, bucket := setup(ctx)
		defer bucket.RemoveAllObjects(ctx)

		file.Write([]byte("abcdef"))

		stopGoroutineChan := make(chan struct{})

		//random calls to .Seek
		go func() {
			for {
				select {
				case <-stopGoroutineChan:
					return
				default:
					file.Seek(rand.Int63n(3), rand.Intn(3))
					p := make([]byte, 10)
					file.Read(p)
				}
			}
		}()

		//random calls to .Seek
		start := time.Now()
		for time.Since(start) < time.Second {
			_, err := file.Seek(rand.Int63n(3), rand.Intn(3))
			if !assert.NoError(t, err) {
				return
			}

			p := make([]byte, 10)
			file.Read(p)
		}

		stopGoroutineChan <- struct{}{}
	})

	t.Run("file should be persisted if dirty when context done", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		file, bucket := setup(ctx)
		defer bucket.RemoveAllObjects(ctx)

		file.Write([]byte("abcdef"))
		assert.True(t, file.content.IsDirty())

		ctx.Cancel()

		time.Sleep(2 * time.Second)
		assert.False(t, file.content.IsDirty())
	})
}
