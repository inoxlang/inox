package binary

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/mod/semver"
)

type releaseInfo struct {
	Assets  []assetInfo `json:"assets"`
	TagName string      `json:"tag_name"`
	Name    string      `json:"name"`
}

func GetLatestNReleases(tags map[string]repoTagInfo, max int) (latestReleases []releaseInfo, err error) {
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

	count := min(len(releaseTagNames), max)
	wg := new(sync.WaitGroup)
	wg.Add(count)

	latestReleases = make([]releaseInfo, count)
	latestReleaseTagNames := releaseTagNames[:count]

	for i, tagName := range latestReleaseTagNames {
		go func(index int, tagName string) {
			defer utils.Recover()

			defer wg.Done()
			release, err := fetchReleaseByTagName(tagName)
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

func fetchReleaseByTagName(tagName string) (data releaseInfo, err error) {
	var endpoint = RELEASE_BY_TAG_API_ENDPOINT + "/" + tagName
	resp, err := http.Get(endpoint)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return
	}

	err = json.Unmarshal(body, &data)
	return
}
