package cloudflareprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
)

const (
	R2_TOKEN_POST_CREATION_DELAY         = 1500 * time.Millisecond
	R2_BUCKET_POST_CREATION_DELAY        = 100 * time.Millisecond
	R2_PERM_GROUPS_FETCH_WAITING_TIMEOUT = 5 * time.Second
)

var (
	ErrNoR2Token = errors.New("no R2 token")
)

type DevSideConfig struct {
	AdditionalTokensApiToken string `json:"additional-tokens-api-token"`
	AccountID                string `json:"account-id"`
}

// TODO: explanaton
type Cloudflare struct {
	lock            sync.Mutex
	highPermsTokens HighPermsCloudflareTokens

	//---- APIs with high permissions ----
	apiTokensApi *cloudflare.API
	//updated each time .highPermsTokens.R2Token is changed, can be nil
	highPermsR2API *cloudflare.API

	singleR2BucketCredentials     map[string]singleR2BucketCredentials
	singleR2BucketCredentialsLock sync.Mutex

	//---- set once ----
	_r2PermGroups       []cloudflare.APITokenPermissionGroups //R2PermGroups() should be used to get the groups
	r2PermGroupsFetched atomic.Bool

	//const
	projectId core.ProjectID
	config    *DevSideConfig
	accountId *cloudflare.ResourceContainer

	//note: cloudflare.API.UserDetails().Account[0].ID is zero
}

func New(projectId core.ProjectID, config *DevSideConfig) (*Cloudflare, error) {
	if config == nil {
		panic(errors.New("cloudflare config should not be nil"))
	}

	apiTokensApi, err := cloudflare.NewWithAPIToken(config.AdditionalTokensApiToken)
	if err != nil {
		return nil, err
	}

	cf := &Cloudflare{
		projectId:                 projectId,
		config:                    config,
		accountId:                 cloudflare.AccountIdentifier(config.AccountID),
		apiTokensApi:              apiTokensApi,
		singleR2BucketCredentials: map[string]singleR2BucketCredentials{},
	}

	go cf.fetchR2PermGroups(1)
	return cf, nil
}

func (c *Cloudflare) TempTokens() HighPermsCloudflareTokens {
	return c.highPermsTokens
}

func (c *Cloudflare) fetchR2PermGroups(maxRetries int) {
	if maxRetries > 0 {
		defer func() {
			if !c.r2PermGroupsFetched.Load() {
				c.fetchR2PermGroups(maxRetries - 1)
			}
		}()
	}

	if c.r2PermGroupsFetched.Load() {
		return
	}

	permGroups, err := c.apiTokensApi.ListAPITokensPermissionGroups(context.Background())

	if err != nil {
		return
	}

	var r2PermGroups []cloudflare.APITokenPermissionGroups

	for _, group := range permGroups {
		if strings.Contains(group.Name, "R2") && !strings.Contains(group.Name, "Read") {
			r2PermGroups = append(r2PermGroups, group)
		}
	}

	if len(r2PermGroups) != 2 {
		return
	}

	c._r2PermGroups = r2PermGroups
	c.r2PermGroupsFetched.Store(true)
}

func (c *Cloudflare) R2PermGroups() []cloudflare.APITokenPermissionGroups {
	start := time.Now()
	for !c.r2PermGroupsFetched.Load() {
		if time.Since(start) >= R2_PERM_GROUPS_FETCH_WAITING_TIMEOUT {
			panic(errors.New("R2 permission groups not fetched"))
		}
		time.Sleep(time.Millisecond)
	}
	return c._r2PermGroups
}

func (c *Cloudflare) getCreateR2TokenNoLock(
	ctx context.Context,
	tokenName string,
	projectId core.ProjectID,

	//optional
	existingTokenId string,
	existingTokenValue string,
) (_id, _value string, _ error) {

	apiTokens, err := c.apiTokensApi.APITokens(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to retrieve API tokens: %w", err)
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
		if tokenId != existingTokenId {
			return "", "", fmt.Errorf("token with name %q should have an id equal to the provided id", tokenName)
		}
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
						"com.cloudflare.api.account." + c.accountId.Identifier: "*",
					},
					PermissionGroups: c.R2PermGroups(),
				},
			},
		}
		var tokenValue string
		if tokenAlreadyExists {
			tokenValue, err = c.apiTokensApi.RollAPIToken(ctx, tokenId)
		} else {
			issueTime := time.Now().Add(-time.Second)
			r2Token.IssuedOn = &issueTime
			r2Token, err = c.apiTokensApi.CreateAPIToken(ctx, r2Token)
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

func (c *Cloudflare) DeleteR2Bucket(ctx *core.Context, bucketToDelete *s3_ns.Bucket) error {
	c.lock.Lock()
	api := c.highPermsR2API
	c.lock.Unlock()

	if api == nil {
		return ErrNoR2Token
	}

	bucketName := bucketToDelete.Name()

	c.singleR2BucketCredentialsLock.Lock()
	delete(c.singleR2BucketCredentials, bucketName)
	c.singleR2BucketCredentialsLock.Unlock()

	buckets, _ := api.ListR2Buckets(ctx, c.accountId, cloudflare.ListR2BucketsParams{})

	for _, bucket := range buckets {
		if bucket.Name == bucketName {
			bucketToDelete.RemoveAllObjects(ctx)
			ctx.Sleep(time.Second)

			return api.DeleteR2Bucket(ctx, c.accountId, bucketToDelete.Name())
		}
	}

	return nil
}

func (c *Cloudflare) CheckBucketExists(ctx *core.Context, bucketName string) (bool, error) {
	c.lock.Lock()
	api := c.highPermsR2API
	c.lock.Unlock()

	return c.CheckBucketExistsNoLock(ctx, bucketName, api)
}

func (c *Cloudflare) CheckBucketExistsNoLock(ctx *core.Context, bucketName string, api *cloudflare.API) (bool, error) {
	buckets, err := api.ListR2Buckets(ctx, c.accountId, cloudflare.ListR2BucketsParams{})

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

func (c *Cloudflare) CreateR2Bucket(ctx *core.Context, bucketName string) error {
	c.lock.Lock()
	api := c.highPermsR2API
	c.lock.Unlock()

	_, err := api.CreateR2Bucket(ctx, c.accountId, cloudflare.CreateR2BucketParameters{
		Name: bucketName,
	})
	time.Sleep(R2_BUCKET_POST_CREATION_DELAY)

	return err
}

func (c *Cloudflare) S3EndpointForR2() core.Host {
	return core.Host("https://" + c.accountId.Identifier + ".r2.cloudflarestorage.com")
}

// singleR2BucketCredentials stores the credentials to access a single bucket.
type singleR2BucketCredentials struct {
	bucket       string
	r2TokenId    string
	r2TokenValue string

	s3Endpoint core.Host

	//S3 credentials computed from r2TokenId & r2TokenValue
	accessKey, secretKey string
}

func (creds singleR2BucketCredentials) AccessKey() string {
	return creds.accessKey
}

func (creds singleR2BucketCredentials) SecretKey() string {
	return creds.secretKey
}

func (creds singleR2BucketCredentials) S3Endpoint() core.Host {
	return creds.s3Endpoint
}

// GetCreateS3CredentialsForSingleBucket creates the bucket bucketName if it does not exist & returns credentials to access the
// bucket.
func (c *Cloudflare) GetCreateS3CredentialsForSingleBucket(
	ctx *core.Context,
	bucketName string,
	projectId core.ProjectID,
) (_ singleR2BucketCredentials, finalErr error) {

	c.singleR2BucketCredentialsLock.Lock()
	creds, ok := c.singleR2BucketCredentials[bucketName]
	c.singleR2BucketCredentialsLock.Unlock()

	if ok {
		return creds, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	api := c.highPermsR2API

	if api == nil {
		return singleR2BucketCredentials{}, ErrMissingHighPermsR2Token
	}

	//create bucket if it does not exist
	{

		exists, err := c.CheckBucketExistsNoLock(ctx, bucketName, api)
		if err != nil {
			finalErr = err
			return
		}

		if !exists {
			_, err := api.CreateR2Bucket(ctx, c.accountId, cloudflare.CreateR2BucketParameters{
				Name: bucketName,
			})
			if err != nil {
				finalErr = err
				return
			}
			time.Sleep(R2_BUCKET_POST_CREATION_DELAY)
		}
	}

	//create token to access the bucket
	r2TokenName := getSingleBucketR2TokenName(bucketName, projectId)

	tokenId, tokenValue, err := c.getCreateR2TokenNoLock(ctx, r2TokenName, projectId, "", "")
	if err != nil {
		return
	}

	accessKey, secretKey := ConvertR2TokenToS3Credentials(tokenId, tokenValue)

	creds = singleR2BucketCredentials{
		bucket:       bucketName,
		accessKey:    accessKey,
		secretKey:    secretKey,
		s3Endpoint:   c.S3EndpointForR2(),
		r2TokenId:    tokenId,
		r2TokenValue: tokenValue,
	}

	c.singleR2BucketCredentialsLock.Lock()
	c.singleR2BucketCredentials[bucketName] = creds
	c.singleR2BucketCredentialsLock.Unlock()

	return creds, nil
}

func getHighPermsR2TokenName(projectId core.ProjectID) string {
	return "R2-" + string(projectId)
}

func getSingleBucketR2TokenName(bucketName string, projectId core.ProjectID) string {
	return "R2-" + bucketName + "-" + string(projectId)
}

func ConvertR2TokenToS3Credentials(tokenId string, tokenValue string) (accessKey, secretKey string) {
	// https://github.com/cloudflare/cloudflare-go/issues/981#issuecomment-1484963748
	accessKey = tokenId
	secretKeyBytes := sha256.Sum256([]byte(tokenValue))
	secretKey = hex.EncodeToString(secretKeyBytes[:])

	return
}
