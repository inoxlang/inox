package project

import (
	"context"
	"errors"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/core"
)

var (
	ErrMissingHighPermsR2Token = errors.New("missing high-permissions R2 token")
)

type HighPermsCloudflareTokens struct {
	//permissions: list/create/delete buckets + any object operations
	R2Token *TempToken `json:"r2Token,omitempty"`
}

func (c *Cloudflare) getUpToDateTempTokens(ctx context.Context) (HighPermsCloudflareTokens, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.getUpToDateTempTokensNoLock(ctx)
}

func (c *Cloudflare) getUpToDateTempTokensNoLock(ctx context.Context) (HighPermsCloudflareTokens, error) {
	var existingTokenId, existingTokenValue string

	if c.highPermsTokens.R2Token != nil {
		existingTokenId = c.highPermsTokens.R2Token.Id
		existingTokenValue = c.highPermsTokens.R2Token.Value
	}

	r2TokenName := getHighPermsR2TokenName(c.projectId)

	tokenId, tokenValue, err := c.getCreateR2TokenNoLock(ctx, r2TokenName, c.projectId, existingTokenId, existingTokenValue)
	if err != nil {
		return HighPermsCloudflareTokens{}, err
	}

	updatedTokens := HighPermsCloudflareTokens{}

	if tokenId == existingTokenId {
		updatedTokens.R2Token = c.highPermsTokens.R2Token
	} else {
		updatedTokens.R2Token = &TempToken{
			Id:    tokenId,
			Value: tokenValue,
		}
	}

	c.highPermsTokens = updatedTokens
	api, err := cloudflare.NewWithAPIToken(tokenValue)
	if err != nil {
		return HighPermsCloudflareTokens{}, err
	}
	c.highPermsR2API = api

	return updatedTokens, nil
}

func (c *Cloudflare) forgetHighPermsTokens(ctx context.Context) {
	c.highPermsTokens = HighPermsCloudflareTokens{}
	c.highPermsR2API = nil
}

func (c *Cloudflare) deleteHighPermsTokens(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	r2TokenId := ""
	if c.highPermsTokens.R2Token != nil {
		r2TokenId = c.highPermsTokens.R2Token.Id
	}

	c.highPermsR2API = nil
	c.highPermsTokens = HighPermsCloudflareTokens{}

	if r2TokenId != "" {
		return c.apiTokensApi.DeleteAPIToken(ctx, r2TokenId)
	}
	return nil
}

func (c *Cloudflare) GetHighPermsS3Credentials(ctx *core.Context) (accessKey, secretKey string, resultOk bool) {
	tokens, err := c.getUpToDateTempTokens(ctx)
	if err != nil {
		return "", "", false
	}
	id := tokens.R2Token.Id
	value := tokens.R2Token.Value

	accessKey, secretKey = ConvertR2TokenToS3Credentials(id, value)

	resultOk = true
	return
}
