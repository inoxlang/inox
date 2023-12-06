package binary

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/inoxlang/inox/internal/utils"
)

var INOX_BINARY_ARCHIVE_GLOB_MATRIX = map[string]map[string]string{
	"linux": {
		"amd64": "*linux-amd64.tar.gz",
	},
}

type assetInfo struct {
	Name                string `json:"name"`
	BrowserDownloadInfo string `json:"browser_download_url"`
}

func getBinaryArchiveGlob() (string, bool) {
	map_ := INOX_BINARY_ARCHIVE_GLOB_MATRIX[runtime.GOOS]
	if map_ == nil {
		return "", false
	}

	glob, ok := map_[runtime.GOARCH]
	return glob, ok
}

func getBinaryArchiveAssetInfo(release releaseInfo) (assetInfo, error) {

	glob, ok := getBinaryArchiveGlob()
	if !ok {
		return assetInfo{}, fmt.Errorf("unsupported: %s x %s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range release.Assets {
		if yes, _ := filepath.Match(glob, asset.Name); yes {
			return asset, nil
		}
	}

	return assetInfo{}, fmt.Errorf("archive not found for release %s (tag %s)", release.Name, release.TagName)
}

func downloadLatestReleaseArchive(outW io.Writer) (*url.URL, []byte, error) {

	tags, err := FetchTags()
	if err != nil {
		return nil, nil, err
	}

	releases, err := GetLatestNReleases(tags, 2)

	if err != nil {
		return nil, nil, err
	}

	if len(releases) == 0 {
		return nil, nil, errors.New("no releases found")
	}

	names := utils.MapSlice(releases, func(r releaseInfo) string { return r.Name })
	fmt.Fprintln(outW, "latest releases =", strings.Join(names, ", "))

	latestRelease := releases[0]

	archiveInfo, err := getBinaryArchiveAssetInfo(latestRelease)
	if err != nil {
		return nil, nil, err
	}

	url, err := url.Parse(archiveInfo.BrowserDownloadInfo)

	if err != nil {
		return nil, nil, fmt.Errorf("invalid download link: %w", err)
	}

	fmt.Fprintln(outW, "download", archiveInfo.BrowserDownloadInfo)

	resp, err := http.Get(archiveInfo.BrowserDownloadInfo)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return nil, nil, err
	}

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, nil, err
	}

	return url, body, nil
}
