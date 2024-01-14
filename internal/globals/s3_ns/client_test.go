package s3_ns

import (
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestS3Client(t *testing.T) {
	if S3_FS_TEST_ACCESS_KEY == "" {
		t.Skip("skip S3 filesystem tests because " + S3_FS_TEST_ACCESS_KEY_ENV_VARNAME + " environment variable is not set")
		return
	}

	t.Run("the request rate limit should be met", func(t *testing.T) {
		//create a context that allows up to one request per second
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{},
			Limits: []core.Limit{
				{
					Name:  OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME,
					Kind:  core.FrequencyLimit,
					Value: 1,
				},
			},
		})
		core.NewGlobalState(ctx)

		bucket, err := OpenBucketWithCredentials(ctx,
			OpenBucketWithCredentialsInput{
				Provider:   "cloudflare",
				BucketName: "test",
				S3Host:     core.Host("s3://bucket-rate-limit"),
				HttpsHost:  core.Host(S3_FS_TEST_ENDPOINT),
				AccessKey:  S3_FS_TEST_ACCESS_KEY,
				SecretKey:  S3_FS_TEST_SECRET_KEY,
			})

		if !assert.NoError(t, err) {
			return
		}

		defer func() {
			bucket.RemoveAllObjects(ctx)
		}()

		_, err = bucket.PutObject(ctx, "obj", strings.NewReader("content"))

		if !assert.NoError(t, err) {
			return
		}

		//wait for token bucket to refill
		time.Sleep(time.Second)

		_, err = bucket.GetObject(ctx, "obj")
		if !assert.NoError(t, err) {
			return
		}

		start := time.Now()

		_, err = bucket.GetObject(ctx, "obj")
		if !assert.NoError(t, err) {
			return
		}

		assert.WithinDuration(t, start.Add(time.Second), time.Now(), 100*time.Millisecond)
	})
}
