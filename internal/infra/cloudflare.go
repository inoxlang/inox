package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/inoxlang/inox/internal/project"
)

type Cloudflare struct {
	R2Token string
}

func GetTempCloudflareTokens(
	ctx context.Context,
	devSideConfig project.DevSideCloudflareConfig,
	tempTokens project.TempCloudflareTokens,
	projectId project.ProjectID,
) (r2tokenValue string, _ error) {
	additionalTokensApiToken := devSideConfig.AdditionalTokensApiToken

	api, err := cloudflare.NewWithAPIToken(additionalTokensApiToken)
	if err != nil {
		return "", err
	}

	permGroups, err := api.ListAPITokensPermissionGroups(ctx)

	if err != nil {
		return "", fmt.Errorf("failed to retrieve API token permission groups: %w", err)
	}

	var r2PermGroups []cloudflare.APITokenPermissionGroups

	for _, group := range permGroups {
		if strings.Contains(group.Name, "R2") {
			r2PermGroups = append(r2PermGroups, group)
		}
	}

	if len(r2PermGroups) < 4 {
		return "", errors.New("failed to retrieve all R2 permission groups")
	}

	//note: api.UserDetails().Account[0].ID is zero
	accountId := devSideConfig.AccountID
	_ = accountId

	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve API tokens: %w", err)
	}

	R2TokenName := GetR2TokenName(projectId)

	R2TokenAlreadyExists := false
	R2TokenExpired := false
	R2TokenId := ""

	for _, token := range apiTokens {
		if token.Name == R2TokenName { //already exists
			if R2TokenAlreadyExists {
				return "", errors.New("R2 API token with duplicate name")
			}
			if token.ExpiresOn != nil {
				R2TokenExpired = token.ExpiresOn.Before(time.Now().Add(time.Hour))
			}

			R2TokenAlreadyExists = true
			R2TokenId = token.ID
		}
	}

	if R2TokenAlreadyExists && tempTokens.R2Token != "" && !R2TokenExpired {
		r2tokenValue = tempTokens.R2Token
	} else {
		//if the token does not exist
		//or is expired
		//or is not present on the developer machine we create/update a token

		//https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/
		//https://developers.cloudflare.com/fundamentals/api/reference/permissions/

		r2Token := cloudflare.APIToken{
			Name: R2TokenName,
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
		if R2TokenAlreadyExists {
			r2Token, err = api.UpdateAPIToken(ctx, R2TokenId, r2Token)
		} else {
			r2Token, err = api.CreateAPIToken(ctx, r2Token)
		}
		if err != nil {
			return "", fmt.Errorf("failed to create R2 API Token: %w", err)
		}
		r2tokenValue = r2Token.Value
	}

	return r2tokenValue, nil
}

func GetR2TokenName(projectId project.ProjectID) string {
	return "R2-" + string(projectId)
}
