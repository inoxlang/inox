package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
) (r2token *project.TempToken, _ error) {
	additionalTokensApiToken := devSideConfig.AdditionalTokensApiToken
	//note: api.UserDetails().Account[0].ID is zero
	accountId := devSideConfig.AccountID

	api, err := cloudflare.NewWithAPIToken(additionalTokensApiToken)
	if err != nil {
		return nil, err
	}

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
			if strings.Contains(group.Name, "R2") {
				r2PermGroups = append(r2PermGroups, group)
			}
		}

		if len(r2PermGroups) < 4 {
			permissionGroupRetrievalError = errors.New("failed to retrieve all R2 permission groups")
			return
		}
	}()

	apiTokens, err := api.APITokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve API tokens: %w", err)
	}

	wg.Wait()
	if permissionGroupRetrievalError != nil {
		return nil, permissionGroupRetrievalError
	}

	R2TokenName := GetR2TokenName(projectId)

	R2TokenAlreadyExists := false
	R2TokenExpired := false
	R2TokenId := ""

	for _, token := range apiTokens {
		if token.Name == R2TokenName { //already exists
			if R2TokenAlreadyExists {
				return nil, errors.New("R2 API token with duplicate name")
			}
			if token.ExpiresOn != nil {
				R2TokenExpired = token.ExpiresOn.Before(time.Now().Add(time.Hour))
			}

			R2TokenAlreadyExists = true
			R2TokenId = token.ID
		}
	}

	if R2TokenAlreadyExists && tempTokens.R2Token != nil && !R2TokenExpired {
		r2token = tempTokens.R2Token
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
		var r2tokenValue string
		if R2TokenAlreadyExists {
			r2tokenValue, err = api.RollAPIToken(ctx, R2TokenId)
		} else {
			r2Token, err = api.CreateAPIToken(ctx, r2Token)
			r2tokenValue = r2Token.Value
			R2TokenId = r2Token.Value
		}
		if err != nil {
			return nil, fmt.Errorf("failed to create R2 API Token: %w", err)
		}

		r2token = &project.TempToken{
			Id:    R2TokenId,
			Value: r2tokenValue,
		}
	}

	return
}

func GetR2TokenName(projectId project.ProjectID) string {
	return "R2-" + string(projectId)
}
