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

type TempCloudflareTokens struct {
	R2Token *TempToken `json:"r2Token,omitempty"`
}

func (t *TempCloudflareTokens) GetS3AccessKeySecretKey() (accessKey, secretKey string, resultOk bool) {
	if t == nil {
		return "", "", false
	}
	//https://github.com/cloudflare/cloudflare-go/issues/981#issuecomment-1484963748
	if t.R2Token == nil || t.R2Token.Id == "" || t.R2Token.Value == "" {
		return "", "", false
	}
	accessKey = t.R2Token.Id

	secretKeyBytes := sha256.Sum256([]byte(t.R2Token.Value))
	secretKey = hex.EncodeToString(secretKeyBytes[:])

	resultOk = true
	return
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
		var cloudflareTempTokens TempCloudflareTokens

		tempTokens, ok := p.getTempTokens()
		if ok && tempTokens.Cloudflare != nil {
			cloudflareTempTokens = *tempTokens.Cloudflare
		}

		r2Token, err := GetTempCloudflareTokens(ctx,
			*cloudflareConfig,
			cloudflareTempTokens,
			p.Id(),
		)
		if err != nil {
			return TempProjectTokens{}, err
		}
		tokens.Cloudflare = &TempCloudflareTokens{
			R2Token: r2Token,
		}
		if p.tempTokens == nil {
			p.tempTokens = &TempProjectTokens{}
		}
		p.tempTokens.Cloudflare = tokens.Cloudflare
	}

	return
}
