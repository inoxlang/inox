package binary

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"

	"github.com/inoxlang/inox/internal/github"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	REPOSITORY  = "denoland/deno"
	RELEASE_TAG = "v1.41.0"
)

type archiveInfo struct {
	name           string
	checksum       string
	binaryChecksum string
}

var ARCHIVE_MATRIX = map[string]map[string]archiveInfo{
	"linux": {
		"amd64": archiveInfo{
			name:           "deno-x86_64-unknown-linux-gnu.zip",
			checksum:       "c9b2620704d4b4f771104e52e5396d03b51a9b5304c4397230ecebe157eca2f3",
			binaryChecksum: "b0877de86c74027327fca3ca37a5ac3780bcc9c70579ecf6c7c9a55d22147aef",
		},
	},
}

func GetArchiveAssetInfo() (github.AssetInfo, archiveInfo, error) {
	releaseInfo, err := github.FetchReleaseByTagName(REPOSITORY, RELEASE_TAG)

	if err != nil {
		return github.AssetInfo{}, archiveInfo{}, err
	}

	if releaseInfo.Name == "" {
		return github.AssetInfo{}, archiveInfo{}, errors.New("unexpected empty release info")
	}

	return getArchiveAssetInfo(releaseInfo)
}

// Download downloads the zip archive for the RELEASE_TAG version of Deno.
func DownloadArchive() (*url.URL, []byte, error) {

	assetInfo, archiveInfo, err := GetArchiveAssetInfo()
	if err != nil {
		return nil, nil, err
	}

	url, err := url.Parse(assetInfo.BrowserDownloadInfo)

	if err != nil {
		return nil, nil, fmt.Errorf("invalid download link: %w", err)
	}

	resp, err := http.Get(assetInfo.BrowserDownloadInfo)
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

	expectedHash := utils.Must(hex.DecodeString(archiveInfo.checksum))
	hash := sha256.Sum256(body)
	if !bytes.Equal(hash[:], expectedHash) {
		return nil, nil, errors.New("downloaded archive does match the ckecksum")
	}

	return url, body, nil
}

func getArchiveAssetInfo(release github.ReleaseInfo) (github.AssetInfo, archiveInfo, error) {

	map_ := ARCHIVE_MATRIX[runtime.GOOS]
	if map_ == nil {
		return github.AssetInfo{}, archiveInfo{}, fmt.Errorf("unsupported: %s x %s", runtime.GOOS, runtime.GOARCH)
	}

	info, ok := map_[runtime.GOARCH]
	if !ok {
		return github.AssetInfo{}, archiveInfo{}, fmt.Errorf("unsupported: %s x %s", runtime.GOOS, runtime.GOARCH)
	}

	for _, asset := range release.Assets {
		if yes, _ := filepath.Match(info.name, asset.Name); yes {
			return asset, info, nil
		}
	}

	return github.AssetInfo{}, archiveInfo{}, fmt.Errorf("archive not found for release %s (tag %s)", release.Name, release.TagName)
}
