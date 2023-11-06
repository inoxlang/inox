package chrome_ns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rs/zerolog"
	"github.com/ysmood/fetchup"
)

// the code in this file is a(n) extraction/rewrite of https://github.com/go-rod/rod/blob/main/lib/launcher/browser.go (MIT license).

const (
	// RevisionDefault for chromium
	RevisionDefault = 1131657

	// RevisionPlaywright for arm linux
	RevisionPlaywright = 1080
)

var (
	BROWSER_BINPATH = ""
)

func SetBrowserBinPath(val string) {
	BROWSER_BINPATH = val
}

// DefaultBrowserDir for downloaded browser. For unix is "$HOME/.cache/inox/browser",
var DefaultBrowserDir = filepath.Join(map[string]string{
	"linux": filepath.Join(os.Getenv("HOME"), ".cache"),
}[runtime.GOOS], "inox", "browser")

// DownloadBrowser downloads a chrome browser, no permissions are checked.
func DownloadBrowser(ctx context.Context, logger zerolog.Logger) (execPath string, finalErr error) {
	revision := RevisionDefault
	dir := filepath.Join(DefaultBrowserDir, fmt.Sprintf("chromium-%d", revision))
	binpath := filepath.Join(dir, "chrome")

	//check the binary is not in the cache.
	if err := Validate(binpath); err == nil {
		logger.Info().Msgf("cache found: %s", binpath)
		return binpath, nil
	}

	logger.Info().Msgf("no chrome browser found, download one")

	hosts := []downloadHost{HostGoogle, HostNPM, HostPlaywright}

	// download browser from the fastest host. It will race downloading a TCP packet from each host and use the fastest host.
	{
		sources := []string{}
		for _, host := range hosts {
			sources = append(sources, host(revision))
		}

		fu := fetchup.New(dir, sources...)
		fu.Ctx = ctx
		fu.Logger = fetchupLogger{logger: logger}

		err := fu.Fetch()
		if err != nil {
			return "", fmt.Errorf("can't find a browser binary for your OS: %w", err)
		}

		err = fetchup.StripFirstDir(dir)
		if err != nil {
			return "", err
		}
	}

	return binpath, nil
}

// Validates checks there is a valid chrome binary at binpath.
func Validate(binpath string) error {
	_, err := os.Stat(binpath)
	if err != nil {
		return err
	}

	cmd := exec.Command(binpath,
		"--headless",
		"--no-sandbox",
		"--disable-gpu",
		"--dump-dom",
		"about:blank",
	)

	b, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(b), "error while loading shared libraries") {
			// When the os is missing some dependencies for chromium we treat it as valid binary.
			return nil
		}

		return fmt.Errorf("failed to run the browser: %w\n%s", err, b)
	}
	if !bytes.Contains(b, []byte(`<html><head></head><body></body></html>`)) {
		return errors.New("the browser executable doesn't support headless mode")
	}

	return nil
}

// LookPath searches for the browser executable from often used paths on current operating system.
func LookPath() (found string, has bool) {
	list := map[string][]string{
		"linux": {
			//chrome
			"chrome",
			"google-chrome",
			"/usr/bin/google-chrome",

			//edge
			"microsoft-edge",
			"/usr/bin/microsoft-edge",

			//chromium
			"chromium",
			"chromium-browser",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/data/data/com.termux/files/usr/bin/chromium-browser",
		},
	}[runtime.GOOS]

	for _, path := range list {
		var err error
		found, err = exec.LookPath(path)
		has = err == nil
		if has {
			break
		}
	}

	return
}

type fetchupLogger struct {
	logger zerolog.Logger
}

func (l fetchupLogger) Println(args ...interface{}) {
	l.logger.Print(args...)
}

// downloadHost formats a revision number to a downloadable URL for the browser.
type downloadHost func(revision int) string

var hostConf = map[string]struct {
	urlPrefix string
	zipName   string
}{
	"linux_amd64": {"Linux_x64", "chrome-linux.zip"},
}[runtime.GOOS+"_"+runtime.GOARCH]

// HostGoogle to download browser
func HostGoogle(revision int) string {
	return fmt.Sprintf(
		"https://storage.googleapis.com/chromium-browser-snapshots/%s/%d/%s",
		hostConf.urlPrefix,
		revision,
		hostConf.zipName,
	)
}

// HostNPM to download browser
func HostNPM(revision int) string {
	return fmt.Sprintf(
		"https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/%s/%d/%s",
		hostConf.urlPrefix,
		revision,
		hostConf.zipName,
	)
}

// HostPlaywright to download browser
func HostPlaywright(revision int) string {
	rev := RevisionPlaywright
	if !(runtime.GOOS == "linux" && runtime.GOARCH == "arm64") {
		rev = revision
	}
	return fmt.Sprintf(
		"https://playwright.azureedge.net/builds/chromium/%d/chromium-linux-arm64.zip",
		rev,
	)
}
