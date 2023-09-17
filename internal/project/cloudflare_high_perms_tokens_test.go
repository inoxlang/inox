package project

import (
	"context"
	"os"
	"strings"
	"testing"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/stretchr/testify/assert"
)

var (
	cloudflareConfig = DevSideCloudflareConfig{
		AdditionalTokensApiToken: os.Getenv("CLOUDFLARE_ADDITIONAL_TOKENS_API_TOKEN"),
		AccountID:                os.Getenv("CLOUDFLARE_ACCOUNT_ID"),
	}
)

func TestGetUpToDateTempCloudflareTokens(t *testing.T) {
	projectId := ProjectID("test-temp-cf-tokens")

	if cloudflareConfig.AdditionalTokensApiToken == "" {
		t.Skip()
		return
	}

	ctx := context.Background()

	cf, err := newCloudflare(projectId, &cloudflareConfig)
	if !assert.NoError(t, err) {
		return
	}

	//cleanup
	if !assert.NoError(t, cf.deleteHighPermsTokens(ctx)) {
		return
	}

	apiTokens, err := cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	tokenCountBeforeTest := len(apiTokens)

	prevTokens, err := cf.getUpToDateTempTokens(ctx)

	if !assert.NoError(t, err) {
		return
	}

	defer deleteTestRelatedTokens(t, ctx, cf.apiTokensApi, projectId)

	prevR2Token := prevTokens.R2Token

	if !assert.NotNil(t, prevR2Token) {
		return
	}

	if !assert.NotEmpty(t, prevR2Token.Value) {
		return
	}

	//if the R2 API token is already present no tokens should be created
	upToToDateTokens, err := cf.getUpToDateTempTokens(ctx)
	r2Token := upToToDateTokens.R2Token

	if !assert.NoError(t, err) {
		return
	}

	assert.Same(t, prevTokens.R2Token, r2Token)

	apiTokens, err = cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}

	cf.forgetHighPermsTokens(ctx)

	//if the R2 API token is not present but the token already exists it should be updated
	upToToDateTokens, err = cf.getUpToDateTempTokens(ctx)
	r2Token = upToToDateTokens.R2Token

	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, r2Token) {
		return
	}

	assert.NotEqual(t, prevR2Token, r2Token)
	assert.Equal(t, prevR2Token.Id, r2Token.Id)
	assert.NotEmpty(t, r2Token.Id)
	assert.NotEmpty(t, r2Token.Value)

	apiTokens, err = cf.apiTokensApi.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}
}

func TestCreateS3CredentialsForSingleBucket(t *testing.T) {
	projectId := ProjectID("test-s3-creds-single-bucket")

	if cloudflareConfig.AdditionalTokensApiToken == "" {
		t.Skip()
		return
	}

	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.Cancel()

	cf, err := newCloudflare(projectId, &cloudflareConfig)
	if !assert.NoError(t, err) {
		return
	}

	//cleanup
	if !assert.NoError(t, cf.deleteHighPermsTokens(ctx)) {
		return
	}

	tokens, err := cf.getUpToDateTempTokens(ctx)
	if !assert.NoError(t, err) {
		return
	}

	defer deleteTestRelatedTokens(t, ctx, cf.apiTokensApi, projectId)

	deleteBucket := func() {
		api, err := cloudflare.NewWithAPIToken(tokens.R2Token.Value)
		if !assert.NoError(t, err) {
			assert.Fail(t, err.Error())
			return
		}
		api.DeleteR2Bucket(ctx, cloudflare.AccountIdentifier(cloudflareConfig.AccountID), "temp")
	}
	deleteBucket()

	//tear down
	defer deleteBucket()

	creds, err := cf.CreateS3CredentialsForSingleBucket(ctx, "temp", projectId)

	if !assert.NoError(t, err) {
		return
	}

	//check that credentials work
	bucket, err := s3_ns.OpenBucketWithCredentials(ctx, s3_ns.OpenBucketWithCredentialsInput{
		S3Host:     "s3://temp",
		Provider:   "cloudflare",
		BucketName: "temp",
		HttpsHost:  creds.s3Endpoint,
		AccessKey:  creds.accessKey,
		SecretKey:  creds.secretKey,
	})

	if !assert.NoError(t, err) {
		return
	}

	defer func() {
		bucket.RemoveAllObjects(ctx)
	}()

	_, err = bucket.PutObject(ctx, "x", strings.NewReader("content"))

	if !assert.NoError(t, err) {
		return
	}

	resp, err := bucket.GetObject(ctx, "x")
	if !assert.NoError(t, err) {
		return
	}

	p, err := resp.ReadAll()

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "content", string(p))
}

func deleteTestRelatedTokens(t *testing.T, ctx context.Context, api *cloudflare.API, projectId ProjectID) {
	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	for _, token := range apiTokens {
		if strings.Contains(token.Name, string(projectId)) {
			err := api.DeleteAPIToken(ctx, token.ID)
			_ = err
		}
	}
}
