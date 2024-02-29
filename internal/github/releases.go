package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/mod/semver"
)

type ReleaseInfo struct {
	Assets  []AssetInfo `json:"assets"`
	TagName string      `json:"tag_name"`
	Name    string      `json:"name"`
}

// GetLatestNReleases returns the information about the latest max (or less) releases.
// The most recent release is the first element.
func GetLatestNReleases(repo string, tags map[string]RepoTagInfo, max int) (latestReleases []ReleaseInfo, err error) {
	if max <= 0 {
		return nil, fmt.Errorf("max should be greater than zero")
	}

	var releaseTagNames []string

	for name, tag := range tags {
		if tag.Version == nil {
			continue
		}
		releaseTagNames = append(releaseTagNames, name)
	}

	semver.Sort(releaseTagNames)

	//v0.1, v0.2 --> v0.2, v.0.1
	slices.Reverse(releaseTagNames)

	count := min(len(releaseTagNames), max)
	wg := new(sync.WaitGroup)
	wg.Add(count)

	latestReleases = make([]ReleaseInfo, count)
	latestReleaseTagNames := releaseTagNames[:count]

	for i, tagName := range latestReleaseTagNames {
		go func(index int, tagName string) {
			defer utils.Recover()

			defer wg.Done()
			release, err := FetchReleaseByTagName(repo, tagName)
			if err == nil {
				latestReleases[index] = release
			}
		}(i, tagName)
	}

	wg.Wait()

	for i, release := range latestReleases {
		if reflect.ValueOf(release).IsZero() {
			latestReleases = nil
			err = fmt.Errorf("failed to fetch data about release %s", latestReleaseTagNames[i])
		}
	}
	return
}

func FetchReleaseByTagName(repo, tagName string) (data ReleaseInfo, err error) {
	var endpoint = strings.ReplaceAll(RELEASE_BY_TAG_API_ENDPOINT_TMPL, "{repo}", repo) + "/" + tagName

	resp, err := http.Get(endpoint)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return
	}

	if resp.StatusCode >= 400 {
		return ReleaseInfo{}, fmt.Errorf("status code %d for GET %s", resp.StatusCode, endpoint)
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return
	}

	err = json.Unmarshal(body, &data)
	return
}
