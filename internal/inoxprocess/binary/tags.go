package binary

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Masterminds/semver/v3"
)

type repoTagInfo struct {
	Name    string          `json:"name"`
	Version *semver.Version //nil if the name is not a valid version
}

func FetchTags() (tags map[string]repoTagInfo, _ error) {
	const ENDPOINT = REPO_TAGS_API_ENDPOINT

	resp, err := http.Get(ENDPOINT)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", ENDPOINT, err)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read body of %s: %w", ENDPOINT, err)
	}

	var tagList []repoTagInfo

	err = json.Unmarshal(body, &tagList)

	if err != nil {
		return nil, err
	}

	tags = make(map[string]repoTagInfo)

	for _, tag := range tagList {
		version, err := semver.NewVersion(tag.Name)
		if err == nil {
			tag.Version = version
		}
		tags[tag.Name] = tag
	}

	return
}
