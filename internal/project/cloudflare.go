package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	R2_TOKEN_POST_CREATION_DELAY = 1500 * time.Millisecond
)

var (
	ErrNoR2Token = errors.New("No R2 token")
)

func getCreateR2Token(
	ctx context.Context,
	tokenName string,
	projectId ProjectID,
	accountId string,
	api *cloudflare.API,
	//optional
	existingTokenId string,
	existingTokenValue string,
) (_id, _value string, _ error) {

	var r2PermGroups []cloudflare.APITokenPermissionGroups
	var permissionGroupRetrievalError error
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		permGroups, err := api.ListAPITokensPermissionGroups(ctx)

		if err != nil {
			permissionGroupRetrievalError = fmt.Errorf("failed to retrieve API token permission groups: %w", err)
			return
		}

		for _, group := range permGroups {
			if strings.Contains(group.Name, "R2") && !strings.Contains(group.Name, "Read") {
				r2PermGroups = append(r2PermGroups, group)
			}
		}

		if len(r2PermGroups) != 2 {
			permissionGroupRetrievalError = errors.New("failed to retrieve R2 permissions")
			return
		}
	}()

	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve API tokens: %w", err)
	}

	wg.Wait()
	if permissionGroupRetrievalError != nil {
		return "", "", permissionGroupRetrievalError
	}

	tokenAlreadyExists := false
	tokenExpired := false
	tokenId := ""

	for _, token := range apiTokens {
		if token.Name == tokenName { //already exists
			if tokenAlreadyExists {
				return "", "", errors.New("R2 API token with duplicate name")
			}
			if token.ExpiresOn != nil {
				tokenExpired = token.ExpiresOn.Before(time.Now().Add(time.Hour))
			}

			tokenAlreadyExists = true
			tokenId = token.ID
		}
	}

	if existingTokenId != "" && existingTokenValue != "" && !tokenExpired {
		return existingTokenId, existingTokenValue, nil
	} else {
		//if the token does not exist
		//or is expired
		//or is not present on the developer machine we create/update a token

		//https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/
		//https://developers.cloudflare.com/fundamentals/api/reference/permissions/

		r2Token := cloudflare.APIToken{
			Name: tokenName,
			Policies: []cloudflare.APITokenPolicies{
				{
					Effect: "allow",
					Resources: map[string]interface{}{
						"com.cloudflare.api.account." + accountId: "*",
					},
					PermissionGroups: r2PermGroups,
				},
			},
		}
		var tokenValue string
		if tokenAlreadyExists {
			tokenValue, err = api.RollAPIToken(ctx, tokenId)
		} else {
			issueTime := time.Now().Add(-time.Second)
			r2Token.IssuedOn = &issueTime
			r2Token, err = api.CreateAPIToken(ctx, r2Token)
			tokenValue = r2Token.Value
			tokenId = r2Token.ID
		}
		if err != nil {
			return "", "", fmt.Errorf("failed to create R2 API Token: %w", err)
		}

		//wait for the token to be valid
		if coreCtx, ok := ctx.(*core.Context); ok {
			coreCtx.Sleep(R2_TOKEN_POST_CREATION_DELAY)
		} else {
			time.Sleep(R2_TOKEN_POST_CREATION_DELAY)
		}

		return tokenId, tokenValue, nil
	}
}

func (p *Project) DeleteSecretsBucket(ctx *core.Context) error {
	tokens := utils.Ret0(p.TempProjectTokens(ctx)).Cloudflare

	bucket, err := p.getCreateSecretsBucket(ctx, false)
	if err != nil {
		return err
	}
	if bucket == nil {
		return nil
	}

	return DeleteR2Bucket(ctx, bucket, *tokens, p.devSideConfig.Cloudflare.AccountID)
}

func DeleteR2Bucket(ctx *core.Context, bucketToDelete *s3_ns.Bucket, tokens TempCloudflareTokens, accountId string) error {
	if tokens.HighPermsR2Token == nil || tokens.HighPermsR2Token.Value == "" {
		return ErrNoR2Token
	}
	api, _ := cloudflare.NewWithAPIToken(tokens.HighPermsR2Token.Value)
	buckets, _ := api.ListR2Buckets(ctx, cloudflare.AccountIdentifier(accountId), cloudflare.ListR2BucketsParams{})

	for _, bucket := range buckets {
		if bucket.Name == bucketToDelete.Name() {
			bucketToDelete.RemoveAllObjects(ctx)
			ctx.Sleep(time.Second)

			return api.DeleteR2Bucket(ctx, cloudflare.AccountIdentifier(accountId), bucketToDelete.Name())
		}
	}

	return nil
}

func checkBucketExists(ctx *core.Context, bucketName string, api *cloudflare.API, accountId string) (bool, error) {
	buckets, err := api.ListR2Buckets(ctx, cloudflare.AccountIdentifier(accountId), cloudflare.ListR2BucketsParams{})

	if err != nil {
		return false, fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			return true, nil
		}
	}

	return false, nil
}

func getHighPermsR2TokenName(projectId ProjectID) string {
	return "R2-" + string(projectId)
}

func getSingleBucketR2TokenName(bucketName string, projectId ProjectID) string {
	return "R2-" + bucketName + "-" + string(projectId)
}

func getS3EndpointForR2(accountId string) core.Host {
	return core.Host("https://" + accountId + ".r2.cloudflarestorage.com")
}
