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
	Cloudflare *HighPermsCloudflareTokens `json:"cloudflare,omitempty"`
}

func (p *Project) TempProjectTokens(ctx *core.Context) (tokens TempProjectTokens, _ error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	return p.TempProjectTokensNoLock(ctx)
}

func (p *Project) TempProjectTokensNoLock(ctx *core.Context) (TempProjectTokens, error) {
	if p.cloudflare != nil {
		upToDateCloudflareTokens, err := p.cloudflare.getUpToDateTempTokens(ctx)
		if err != nil {
			return TempProjectTokens{}, err
		}

		if p.tempTokens == nil {
			p.tempTokens = &TempProjectTokens{}
		}
		p.tempTokens.Cloudflare = &upToDateCloudflareTokens
		return *p.tempTokens, nil
	}

	return TempProjectTokens{}, nil
}

func ConvertR2TokenToS3Credentials(tokenId string, tokenValue string) (accessKey, secretKey string) {
	// https://github.com/cloudflare/cloudflare-go/issues/981#issuecomment-1484963748
	accessKey = tokenId
	secretKeyBytes := sha256.Sum256([]byte(tokenValue))
	secretKey = hex.EncodeToString(secretKeyBytes[:])

	return
}
