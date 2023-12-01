package project

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/cloudflareprovider"
)

type TempProjectTokens struct {
	Cloudflare *cloudflareprovider.HighPermsCloudflareTokens `json:"cloudflare,omitempty"`
}

func (p *Project) TempProjectTokens(ctx *core.Context) (tokens TempProjectTokens, _ error) {
	closestState := ctx.GetClosestState()
	p.lock.Lock(closestState, p)
	defer p.lock.Unlock(closestState, p)

	return p.TempProjectTokensNoLock(ctx)
}

func (p *Project) TempProjectTokensNoLock(ctx *core.Context) (TempProjectTokens, error) {
	if p.cloudflare != nil {
		upToDateCloudflareTokens, err := p.cloudflare.GetUpToDateTempTokens(ctx)
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
