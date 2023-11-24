package systemd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	inoxdcrypto "github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/inoxd/systemd/unitenv"
)

const (
	DEFAULT_INOXD_ENV_FILE_PATH = "/run/inoxd/env"
	INOXD_ENV_FILE_PERMS        = fs.FileMode(0o440)
	INOXD_ENV_FILE_DIR_PERMS    = fs.FileMode(0o770)
)

var (
	ErrBadEnvFilePerms = errors.New(
		"the permissions of the inoxd environment file are not " + strconv.FormatUint(uint64(INOXD_ENV_FILE_PERMS), 8))
	ErrEnvFileNotOwnedByRoot      = errors.New("the inoxd environment file is not owned by root")
	ErrEnvFileNotOwnedByRootGroup = errors.New("the inoxd environment file is not owned by the root group")
)

type EnvFileCreationParams struct {
	CloudflareOriginCertificate string //optional, if set CLOUDFLARE_ORIGIN_CERTIFICATE is set.
	TunnelProviderName          string
	ExposeProjectServers        bool
	ExposeWebservers            bool
}

// CreateInoxdEnvFileIfNotExists creates an environment file to be used by systemd to start inoxd.
// The file contains EXTREMELY SENSITIVE information:
// INOXD_MASTER_KEYSET: a set of master keys primarily used to encrypt and decrypt keys.
// CLOUDFLARE_ORIGIN_CERTIFICATE: the origin certificate delivered by Cloudflare.
func CreateInoxdEnvFileIfNotExists(outW io.Writer, input EnvFileCreationParams) (path string, _ error) {
	if input.TunnelProviderName != "" && input.ExposeProjectServers || input.ExposeWebservers {
		return "", errors.New("invalid arguments")
	}

	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	if currentUser.Uid != "0" {
		return "", ErrNotRoot
	}

	path = DEFAULT_INOXD_ENV_FILE_PATH
	info, err := os.Stat(path)

	switch {
	default: //already exists
		fmt.Fprintln(outW, "inoxd environment file already exists: "+path)

		//check permissions, owner and group.

		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			UID := int(stat.Uid)
			GID := int(stat.Gid)

			if UID != 0 {
				return "", fmt.Errorf("%w (file %s)", ErrEnvFileNotOwnedByRoot, path)
			}
			if GID != 0 {
				return "", fmt.Errorf("%w (file %s)", ErrEnvFileNotOwnedByRootGroup, path)
			}
		} else {
			return "", fmt.Errorf("failed to get owner of %s: not on linux", path)
		}

		if info.Mode().Perm() != INOXD_ENV_FILE_PERMS {
			return "", fmt.Errorf("%w (file %s)", ErrBadEnvFilePerms, path)
		}

		return
	case os.IsNotExist(err):
		//create writable file

		if err := os.MkdirAll(filepath.Dir(path), INOXD_ENV_FILE_DIR_PERMS); err != nil {
			return "", err
		}

		perms := fs.FileMode(INOXD_ENV_FILE_PERMS | 0o200) //allow write

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, perms)
		if err != nil {
			return "", err
		}

		//write environment variables to the file

		addStringVariable(f, unitenv.INOXD_MASTER_KEYSET_ENV_VARNAME, inoxdcrypto.GenerateRandomInoxdMasterKeyset())
		if input.CloudflareOriginCertificate != "" {
			//encode to base64 to avoid having linefeeds and carriage returns.
			varValue := base64.StdEncoding.EncodeToString([]byte(input.CloudflareOriginCertificate))
			addStringVariable(f, unitenv.CLOUDFLARE_ORIGIN_CERTIFICATE_ENV_VARNAME, varValue)
		}
		if input.TunnelProviderName != "" {
			addStringVariable(f, unitenv.TUNNEL_PROVIDER_ENV_VARNAME, input.TunnelProviderName)
		}

		if input.ExposeProjectServers {
			addBooleanVariable(f, unitenv.EXPOSE_PROJECT_SERVERS_ENV_VARNAME, input.ExposeProjectServers)
		}

		if input.ExposeWebservers {
			addBooleanVariable(f, unitenv.EXPOSE_WEB_SERVERS_ENV_VARNAME, input.ExposeWebservers)
		}

		f.Close()

		//remove write permission
		err = os.Chmod(path, INOXD_ENV_FILE_PERMS)
		if err != nil {
			return "", err
		}

		return
	case err != nil: //unexpected error
		return "", fmt.Errorf("failed to get info about %s: %w", path, err)
	}

}

func addStringVariable[S ~string](f *os.File, name string, value S) {
	fmt.Fprintf(f, "%s='%s'\n", name, value)
}

func addBooleanVariable(f *os.File, name string, value bool) {
	fmt.Fprintf(f, "%s=%t\n", name, value)
}
