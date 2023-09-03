package project

import (
	"context"
	"crypto/sha256"
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

func (t TempCloudflareTokens) GetS3AccessKeySecretKey() (accessKey, secretKey string, _ bool) {
	if t.R2Token == nil || t.R2Token.Id == "" || t.R2Token.Value == "" {
		return "", "", false
	}
	accessKeyBytes := sha256.Sum256([]byte(t.R2Token.Id))
	accessKey = string(accessKeyBytes[:])

	secretKeyBytes := sha256.Sum256([]byte(t.R2Token.Value))
	secretKey = string(secretKeyBytes[:])
	return
}

func (p *Project) getTempTokens() (TempProjectTokens, bool) {
	if p.tempTokens != nil {
		return *p.tempTokens, true
	}
	return TempProjectTokens{}, false
}

func (p *Project) TempProjectTokens(ctx context.Context) (tokens TempProjectTokens, _ error) {
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
	}

	return
}
