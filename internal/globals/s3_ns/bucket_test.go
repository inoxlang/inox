package s3_ns

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

var (
	S3_HOST_RESOLUTION_DATA_WITHOUT_CREDENTIALS = core.NewObjectFromMapNoInit(core.ValMap{
		"bucket":   core.Str("test"),
		"provider": core.Str("cloudflare"),
	})

	S3_HOST_RESOLUTION_DATA_WITH_CREDENTIALS = core.NewObjectFromMapNoInit(core.ValMap{
		"bucket":     core.Str("test"),
		"host":       core.Host(S3_FS_TEST_ENDPOINT),
		"provider":   core.Str("cloudflare"),
		"access-key": core.Str(S3_FS_TEST_ACCESS_KEY),
		"secret-key": utils.Must(core.SECRET_STRING_PATTERN.NewSecret(
			core.NewContexWithEmptyState(core.ContextConfig{}, nil),
			S3_FS_TEST_SECRET_KEY,
		)),
	})
)

func TestOpenBucket(t *testing.T) {
	if S3_FS_TEST_ACCESS_KEY == "" {
		t.Skip()
		return
	}

	t.Run("no options", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			HostResolutions: map[core.Host]core.Value{
				"s3://bucket": S3_HOST_RESOLUTION_DATA_WITH_CREDENTIALS,
			},
		})
		state := core.NewGlobalState(ctx)
		state.Project = &testProject{id: core.RandomProjectID("test-open-bucket-no-options")}
		state.MainState = state

		bucket, err := OpenBucket(ctx, "s3://bucket", OpenBucketOptions{})

		bucket.Close()

		if !assert.NoError(t, err) {
			return
		}

		//test write then read
		_, err = bucket.PutObject(ctx, "x", strings.NewReader("content"))
		if !assert.NoError(t, err) {
			return
		}

		resp, err := bucket.GetObject(ctx, "x")
		if !assert.NoError(t, err) {
			return
		}

		content, err := resp.ReadAll()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []byte("content"), content)
	})

	t.Run("credentials got from project", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			HostResolutions: map[core.Host]core.Value{
				"s3://bucket": S3_HOST_RESOLUTION_DATA_WITHOUT_CREDENTIALS,
			},
		})
		state := core.NewGlobalState(ctx)
		state.MainState = state

		bucket, err := OpenBucket(ctx, "s3://bucket", OpenBucketOptions{
			AllowGettingCredentialsFromProject: true,
			Project:                            &testProject{id: core.RandomProjectID("test-open-bucket-creds-from-project")},
		})

		if !assert.NoError(t, err) {
			return
		}

		bucket.Close()

		if !assert.NoError(t, err) {
			return
		}

		//test write then read
		_, err = bucket.PutObject(ctx, "x", strings.NewReader("content"))
		if !assert.NoError(t, err) {
			return
		}

		resp, err := bucket.GetObject(ctx, "x")
		if !assert.NoError(t, err) {
			return
		}

		content, err := resp.ReadAll()
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, []byte("content"), content)
	})
}

type testProject struct {
	id core.ProjectID
}

func (p *testProject) Id() core.ProjectID {
	return p.id
}

func (p *testProject) BaseImage() (core.Image, error) {
	return nil, core.ErrNotImplemented
}

func (*testProject) GetS3CredentialsForBucket(
	ctx *core.Context,
	bucketName string,
	provider string,
) (accessKey string, secretKey string, _ core.Host, _ error) {
	return S3_FS_TEST_ACCESS_KEY, S3_FS_TEST_SECRET_KEY, core.Host(S3_FS_TEST_ENDPOINT), nil
}

func (*testProject) CanProvideS3Credentials(s3Provider string) (bool, error) {
	return true, nil
}
