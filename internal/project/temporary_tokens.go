package project

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/inoxlang/inox/internal/core"
)

type TempToken struct {
	Id    string `json:"id"`
	Value string `json:"value"`
}

type TempProjectTokens struct {
	Cloudflare *TempCloudflareTokens `json:"cloudflare,omitempty"`
}

func (p *Project) getTempTokens() (TempProjectTokens, bool) {
	if p.tempTokens != nil {
		return *p.tempTokens, true
	}
	return TempProjectTokens{}, false
}

func (p *Project) TempProjectTokens(ctx *core.Context) (tokens TempProjectTokens, _ error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	cloudflareConfig := p.DevSideConfig().Cloudflare
	if cloudflareConfig != nil {
		var cloudflareTempTokens *TempCloudflareTokens

		tempTokens, ok := p.getTempTokens()
		if ok && tempTokens.Cloudflare != nil {
			cloudflareTempTokens = tempTokens.Cloudflare
		}

		upToDateCloudflareTokens, err := getUpToDateTempCloudflareTokens(ctx,
			*cloudflareConfig,
			cloudflareTempTokens,
			p.Id(),
		)
		if err != nil {
			return TempProjectTokens{}, err
		}
		tokens.Cloudflare = upToDateCloudflareTokens
		if p.tempTokens == nil {
			p.tempTokens = &TempProjectTokens{}
		}
		p.tempTokens.Cloudflare = tokens.Cloudflare
	}

	return
}

func ConvertR2TokenToS3Credentials(tokenId string, tokenValue string) (accessKey, secretKey string) {
	// https://github.com/cloudflare/cloudflare-go/issues/981#issuecomment-1484963748
	accessKey = tokenId
	secretKeyBytes := sha256.Sum256([]byte(tokenValue))
	secretKey = hex.EncodeToString(secretKeyBytes[:])

	return
}
