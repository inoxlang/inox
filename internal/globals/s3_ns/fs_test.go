package s3_ns

import (
	"math/rand"
	"os"
	"strconv"
	"testing"

	"gopkg.in/check.v1"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
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
	bucket         *Bucket
	creatorContext *core.Context
}

type S3FsWithSecondaryContextTestSuite struct {
	S3FsTestSuite
	secondaryContext *core.Context
}

func (s *S3FsTestSuite) setUpTest(c *check.C) {
	testId := strconv.Itoa(int(rand.Int31()))
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	bucket, err := OpenBucketWithCredentials(ctx,
		OpenBucketWithCredentialsInput{
			Provider:   "cloudflare",
			BucketName: "test",
			S3Host:     core.Host("s3://bucket-" + testId),
			HttpsHost:  core.Host(S3_FS_TEST_ENDPOINT),
			AccessKey:  S3_FS_TEST_ACCESS_KEY,
			SecretKey:  S3_FS_TEST_SECRET_KEY,
		})
	if err != nil {
		c.Fatal(err)
		return
	}
	s.bucket = bucket
	s.creatorContext = ctx

}

func (s *S3FsTestSuite) SetUpTest(c *check.C) {
	s.setUpTest(c)
	s.BasicTestSuite = fs_ns.BasicTestSuite{
		FS: NewS3Filesystem(s.creatorContext, s.bucket),
	}
}

func (s *S3FsTestSuite) TearDownTest(c *check.C) {
	s.bucket.RemoveAllObjects(s.creatorContext)
}

func (s *S3FsWithSecondaryContextTestSuite) SetUpTest(c *check.C) {
	s.S3FsTestSuite.setUpTest(c)
	s.secondaryContext = core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	s.BasicTestSuite = fs_ns.BasicTestSuite{
		FS: NewS3Filesystem(s.creatorContext, s.bucket).WithSecondaryContext(s.secondaryContext).(*S3Filesystem),
	}
}

func TestFilesystemWithSecondaryContext(t *testing.T) {
	if S3_FS_TEST_ACCESS_KEY == "" {
		t.Skip("skip S3 filesystem tests because " + S3_FS_TEST_ACCESS_KEY_ENV_VARNAME + " environment variable is not set")
		return
	}

	result := check.Run(&S3FsWithSecondaryContextTestSuite{}, &check.RunConf{
		Verbose: testing.Verbose(),
	})

	if result.Failed > 0 || result.Panicked > 0 {
		assert.Fail(t, result.String())
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
