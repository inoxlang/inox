package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type RepoTagInfo struct {
	Name    string          `json:"name"`
	Version *semver.Version //nil if the name is not a valid version
}

func FetchTags(repo string) (tags map[string]RepoTagInfo, _ error) {
	endpoint := strings.ReplaceAll(REPO_TAGS_API_ENDPOINT_TMPL, "{repo}", repo)

	resp, err := http.Get(endpoint)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", endpoint, err)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read body of %s: %w", endpoint, err)
	}

	var tagList []RepoTagInfo

	err = json.Unmarshal(body, &tagList)

	if err != nil {
		return nil, err
	}

	tags = make(map[string]RepoTagInfo)

	for _, tag := range tagList {
		version, err := semver.NewVersion(tag.Name)
		if err == nil {
			tag.Version = version
		}
		tags[tag.Name] = tag
	}

	return
}
