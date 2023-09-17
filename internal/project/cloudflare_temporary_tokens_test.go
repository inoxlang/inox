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
	projectId := ProjectID("test-temp-cr-tokens")

	if cloudflareConfig.AdditionalTokensApiToken == "" {
		t.Skip()
		return
	}

	ctx := context.Background()

	api, err := cloudflare.NewWithAPIToken(cloudflareConfig.AdditionalTokensApiToken)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	defer deleteTestRelatedTokens(t, ctx, api, projectId)
	deleteTestRelatedTokens(t, ctx, api, projectId)

	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	tokenCountBeforeTest := len(apiTokens)

	prevTokens, err := getUpToDateTempCloudflareTokens(ctx, cloudflareConfig, &TempCloudflareTokens{}, projectId)
	prevR2Token := prevTokens.HighPermsR2Token

	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, prevR2Token) {
		return
	}

	if !assert.NotEmpty(t, prevR2Token.Value) {
		return
	}

	//if a R2 API token is passed no tokens should be created
	upToToDateTokens, err := getUpToDateTempCloudflareTokens(ctx, cloudflareConfig, &TempCloudflareTokens{
		HighPermsR2Token: prevR2Token,
	}, projectId)
	r2Token := upToToDateTokens.HighPermsR2Token

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, prevTokens.HighPermsR2Token, r2Token)

	apiTokens, err = api.APITokens(ctx)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	if !assert.Equal(t, 1+tokenCountBeforeTest, len(apiTokens)) {
		return
	}

	//if no R2 API token is passed and the token already exists it should be updated
	upToToDateTokens, err = getUpToDateTempCloudflareTokens(ctx, cloudflareConfig, &TempCloudflareTokens{}, projectId)
	r2Token = upToToDateTokens.HighPermsR2Token

	if !assert.NoError(t, err) {
		return
	}

	if !assert.NotNil(t, r2Token) {
		return
	}

	assert.NotEqual(t, prevR2Token, r2Token)
	assert.NotEmpty(t, r2Token.Id)
	assert.NotEmpty(t, r2Token.Value)

	apiTokens, err = api.APITokens(ctx)
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

	api, err := cloudflare.NewWithAPIToken(cloudflareConfig.AdditionalTokensApiToken)
	if err != nil {
		assert.Fail(t, err.Error())
		return
	}

	defer deleteTestRelatedTokens(t, ctx, api, projectId)
	deleteTestRelatedTokens(t, ctx, api, projectId)

	tokens, err := getUpToDateTempCloudflareTokens(ctx, cloudflareConfig, &TempCloudflareTokens{}, projectId)
	if !assert.NoError(t, err) {
		return
	}

	deleteBucket := func() {
		api, err := cloudflare.NewWithAPIToken(tokens.HighPermsR2Token.Value)
		if !assert.NoError(t, err) {
			assert.Fail(t, err.Error())
			return
		}
		api.DeleteR2Bucket(ctx, cloudflare.AccountIdentifier(cloudflareConfig.AccountID), "temp")
	}
	deleteBucket()

	//tear down
	defer deleteBucket()

	accessKey, secretKey, s3Endpoint, err := tokens.CreateS3CredentialsForSingleBucket(ctx, "temp", projectId, cloudflareConfig)

	if !assert.NoError(t, err) {
		return
	}

	//check that credentials work
	bucket, err := s3_ns.OpenBucketWithCredentials(ctx, s3_ns.OpenBucketWithCredentialsInput{
		S3Host:     "s3://temp",
		Provider:   "cloudflare",
		BucketName: "temp",
		HttpsHost:  s3Endpoint,
		AccessKey:  accessKey,
		SecretKey:  secretKey,
	})

	if !assert.NoError(t, err) {
		return
	}

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
