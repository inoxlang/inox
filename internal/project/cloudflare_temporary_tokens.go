package project

import (
	"context"
	"errors"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrMissingHighPermsR2Token = errors.New("missing high-permissions R2 token")
)

type TempCloudflareTokens struct {
	//R2 token with high permissions (list/create buckets + any object operations)
	HighPermsR2Token *TempToken `json:"r2Token,omitempty"`
}

func getUpToDateTempCloudflareTokens(
	ctx context.Context,

	devSideConfig DevSideCloudflareConfig,

	//optional
	currentTempTokens *TempCloudflareTokens,

	projectId ProjectID,
) (*TempCloudflareTokens, error) {
	additionalTokensApiToken := devSideConfig.AdditionalTokensApiToken
	//note: api.UserDetails().Account[0].ID is zero
	accountId := devSideConfig.AccountID

	api, err := cloudflare.NewWithAPIToken(additionalTokensApiToken)
	if err != nil {
		return nil, err
	}

	var existingTokenId, existingTokenValue string
	if currentTempTokens != nil && currentTempTokens.HighPermsR2Token != nil {
		existingTokenId = currentTempTokens.HighPermsR2Token.Id
		existingTokenValue = currentTempTokens.HighPermsR2Token.Value
	}

	r2TokenName := getHighPermsR2TokenName(projectId)

	tokenId, tokenValue, err := getCreateR2Token(ctx, r2TokenName, projectId, accountId, api, existingTokenId, existingTokenValue)
	if err != nil {
		return nil, err
	}

	return &TempCloudflareTokens{
		HighPermsR2Token: &TempToken{
			Id:    tokenId,
			Value: tokenValue,
		},
	}, nil
}

func (t *TempCloudflareTokens) GetHighPermsS3Credentials() (accessKey, secretKey string, resultOk bool) {
	if t == nil {
		return "", "", false
	}
	if t.HighPermsR2Token == nil || t.HighPermsR2Token.Id == "" || t.HighPermsR2Token.Value == "" {
		return "", "", false
	}

	accessKey, secretKey = ConvertR2TokenToS3Credentials(t.HighPermsR2Token.Id, t.HighPermsR2Token.Value)

	resultOk = true
	return
}

// CreateS3CredentialsForSingleBucket creates the bucket bucketName if it does not exist & returns credentials to access the
// bucket.
func (t *TempCloudflareTokens) CreateS3CredentialsForSingleBucket(
	ctx *core.Context,
	bucketName string,
	projectId ProjectID,
	cloudflareConfig DevSideCloudflareConfig,
) /*results*/ (accessKey, secretKey string, s3Endpoint core.Host, _ error) {

	accountId := cloudflareConfig.AccountID

	if t.HighPermsR2Token == nil || t.HighPermsR2Token.Id == "" || t.HighPermsR2Token.Value == "" {
		return "", "", "", ErrMissingHighPermsR2Token
	}

	//create bucket if it does not exist
	{
		api := utils.Must(cloudflare.NewWithAPIToken(t.HighPermsR2Token.Value))

		exists, err := checkBucketExists(ctx, bucketName, api, accountId)
		if err != nil {
			return "", "", "", err
		}

		if !exists {
			_, err := api.CreateR2Bucket(ctx, cloudflare.AccountIdentifier(accountId), cloudflare.CreateR2BucketParameters{
				Name: bucketName,
			})
			if err != nil {
				return "", "", "", err
			}
		}
	}

	//create token to access the bucket
	api := utils.Must(cloudflare.NewWithAPIToken(cloudflareConfig.AdditionalTokensApiToken))
	r2TokenName := getSingleBucketR2TokenName(bucketName, projectId)

	tokenId, tokenValue, err := getCreateR2Token(ctx, r2TokenName, projectId, accountId, api, "", "")
	if err != nil {
		return
	}

	accessKey, secretKey = ConvertR2TokenToS3Credentials(tokenId, tokenValue)
	s3Endpoint = getS3EndpointForR2(accountId)
	return
}
