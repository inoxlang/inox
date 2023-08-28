package infra

import (
	"context"

	"github.com/inoxlang/inox/internal/project"
)

func GetTempProjectTokens(ctx context.Context, proj *project.Project) (tokens project.TempProjectTokens, _ error) {
	if proj.DevSideConfig().Cloudflare != nil {
		var cloudflareTempTokens project.TempCloudflareTokens

		tempTokens, ok := proj.TempTokens()
		if ok && tempTokens.Cloudflare != nil {
			cloudflareTempTokens = *tempTokens.Cloudflare
		}

		r2Token, err := GetTempCloudflareTokens(ctx,
			*proj.DevSideConfig().Cloudflare,
			cloudflareTempTokens,
			proj.Id(),
		)
		if err != nil {
			return project.TempProjectTokens{}, err
		}
		tokens.Cloudflare = &project.TempCloudflareTokens{
			R2Token: r2Token,
		}
	}

	return
}
