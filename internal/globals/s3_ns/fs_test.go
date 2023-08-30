package s3_ns

import (
	"math/rand"
	"os"
	"strconv"
	"testing"

	"gopkg.in/check.v1"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
)

var (
	S3_FS_TEST_ACCESS_KEY_ENV_VARNAME = "S3_FS_TEST_ACCESS_KEY"
	S3_FS_TEST_ACCESS_KEY             = os.Getenv(S3_FS_TEST_ACCESS_KEY_ENV_VARNAME)
	S3_FS_TEST_SECRET_KEY             = os.Getenv("S3_FS_TEST_SECRET_KEY")
	S3_FS_TEST_ENDPOINT               = os.Getenv("S3_FS_TEST_ENDPOINT")
	_                                 = check.Suite(&S3FsTestSuite{})
)

type S3FsTestSuite struct {
	fs_ns.BasicTestSuite
	bucket *Bucket
	ctx    *core.Context
}

func (s *S3FsTestSuite) SetUpTest(c *check.C) {
	testId := strconv.Itoa(int(rand.Int31()))
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	bucket, err := openBucketWithCredentials(ctx,
		openBucketWithCredentialsInput{
			provider:   "cloudflare",
			bucketName: "test",
			s3Host:     core.Host("s3://bucket-" + testId),
			httpsHost:  core.Host(S3_FS_TEST_ENDPOINT),
			accessKey:  S3_FS_TEST_ACCESS_KEY,
			secretKey:  S3_FS_TEST_SECRET_KEY,
		})
	if err != nil {
		c.Fatal(err)
		return
	}
	s.bucket = bucket
	s.ctx = ctx

	s.BasicTestSuite = fs_ns.BasicTestSuite{
		FS: NewS3Filesystem(ctx, bucket),
	}
}

func (s *S3FsTestSuite) TearDownTest(c *check.C) {
	//remove all objects

	objectChan := s.bucket.client.libClient.ListObjects(s.ctx, "test", minio.ListObjectsOptions{Recursive: true})
	for range s.bucket.client.libClient.RemoveObjects(s.ctx, "test", objectChan, minio.RemoveObjectsOptions{}) {
	}
}

func TestFilesystem(t *testing.T) {
	if S3_FS_TEST_ACCESS_KEY == "" {
		t.Skip("skip S3 filesystem tests because " + S3_FS_TEST_ACCESS_KEY_ENV_VARNAME + " environment variable is not set")
		return
	}

	result := check.Run(&S3FsTestSuite{}, &check.RunConf{
		Verbose: testing.Verbose(),
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
	}
}
