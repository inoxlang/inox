package metricsperf

import (
	"os"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/stretchr/testify/assert"
)

var (
	METRICS_PERF_TEST_ACCESS_KEY_ENV_VARNAME = "METRICS_PERF_TEST_ACCESS_KEY"
	METRICS_PERF_TEST_ACCESS_KEY             = os.Getenv(METRICS_PERF_TEST_ACCESS_KEY_ENV_VARNAME)
	METRICS_PERF_TEST_SECRET_KEY             = os.Getenv("METRICS_PERF_TEST_SECRET_KEY")
	METRICS_PERF_TEST_ENDPOINT               = os.Getenv("S3_FS_TEST_ENDPOINT")
)

func TestStartPeriodicPerfProfilesCollection(t *testing.T) {
	if METRICS_PERF_TEST_ACCESS_KEY == "" {
		t.Skip("skip S3 filesystem tests because " + METRICS_PERF_TEST_ACCESS_KEY_ENV_VARNAME + " environment variable is not set")
		return
	}

	bucketConf := s3_ns.OpenBucketWithCredentialsInput{
		Provider:   "cloudflare",
		BucketName: "metrics-perf-test",
		HttpsHost:  core.Host(METRICS_PERF_TEST_ENDPOINT),
		AccessKey:  METRICS_PERF_TEST_ACCESS_KEY,
		SecretKey:  METRICS_PERF_TEST_SECRET_KEY,
	}

	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	s3Client, err := s3_ns.OpenBucketWithCredentials(ctx, bucketConf)

	if !assert.NoError(t, err) {
		return
	}

	s3Client.RemoveAllObjects(ctx)
	defer func() {
		s3Client.RemoveAllObjects(ctx)
	}()

	stop, err := StartPeriodicPerfProfilesCollection(ctx, PerfDataCollectionConfig{
		ProfileSavePeriod: MIN_SAVE_PERIOD,
		Bucket:            bucketConf,
	})

	if !assert.NoError(t, err) {
		return
	}

	defer close(stop)

	stop <- struct{}{}

	//wait for the objects to be visible
	time.Sleep(100 * time.Millisecond)

	objects, err := s3Client.ListObjects(ctx, "")
	if !assert.NoError(t, err) {
		return
	}

	assert.Len(t, objects, 2)
}
