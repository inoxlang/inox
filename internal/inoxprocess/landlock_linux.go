//go:build linux

package inoxprocess

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/shoenig/go-landlock"
)

var (
	stdioPaths []*landlock.Path
)

func init() {
	//initialize stdioPaths.
	//directly using landlock.Stdio() causes locking to return EBADFD when stdin is not available.

	paths := []*landlock.Path{
		landlock.File("/dev/full", "rw"),
		landlock.File("/dev/zero", "r"),
		landlock.File("/dev/fd", "r"),
		landlock.File("/dev/stdin", "rw"),
		landlock.File("/dev/stdout", "rw"),
		landlock.File("/dev/urandom", "r"),
		landlock.Dir("/dev/log", "w"),
		landlock.Dir("/usr/share/locale", "r"),
		landlock.File("/proc/self/cmdline", "r"),
		landlock.File("/usr/share/zoneinfo", "r"),
		landlock.File("/usr/share/common-licenses", "r"),
		landlock.File("/proc/sys/kernel/ngroups_max", "r"),
		landlock.File("/proc/sys/kernel/cap_last_cap", "r"),
		landlock.File("/proc/sys/vm/overcommit_memory", "r"),
	}

	for _, p := range paths {
		var fsPath string
		var colonCount = 0
		s := p.String()

		for i, c := range s {
			if c == ':' {
				colonCount++
			}
			if colonCount == 3 {
				fsPath = s[i+1:]
			}
		}
		if _, err := os.Stat(fsPath); err == nil {
			stdioPaths = append(stdioPaths, p)
		}
	}
}

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission, fls *fs_ns.OsFilesystem, additionalPaths []*landlock.Path) {
	allowedPaths := []*landlock.Path{landlock.VMInfo(), landlock.Shared()}
	allowedPaths = append(allowedPaths, stdioPaths...)
	allowedPaths = append(allowedPaths, additionalPaths...)

	var allowDNS, allowCerts bool

	executablePaths := map[string]struct{}{}
	dirPaths := map[string]map[permkind.PermissionKind]struct{}{}
	filePaths := map[string]map[permkind.PermissionKind]struct{}{}

	for _, perm := range grantedPerms {
		switch p := perm.(type) {
		case core.DNSPermission:
			allowDNS = true
		case core.WebsocketPermission:
			allowCerts = true
		case core.HttpPermission:
			allowCerts = true
			allowDNS = true
		case core.CommandPermission:
			allowedPath := getLandlockPathForCommandPermission(p, executablePaths, fls)
			if allowedPath != nil {
				allowedPaths = append(allowedPaths, allowedPath)
			}
		case core.FilesystemPermission:
			updateFileAndDirLandlockPaths(p, filePaths, dirPaths, fls)
		}
	}

	getMode := func(kinds map[core.PermissionKind]struct{}, isDir bool) string {
		read := false
		write := false
		create := false

		for kind := range kinds {
			switch kind {
			case permkind.Read:
				read = true
			case permkind.Write:
				write = true
				create = true
			case permkind.Delete:
				write = true
			}
		}

		s := ""
		if read {
			s += "r"
		}
		if write {
			s += "w"
		}
		if create {
			s += "c"
		}
		return s
	}

	for path, kinds := range dirPaths {
		allowedPaths = append(allowedPaths, landlock.Dir(path, getMode(kinds, true)))
	}

	for path, kinds := range filePaths {
		allowedPaths = append(allowedPaths, landlock.File(path, getMode(kinds, false)))
	}

	if allowDNS {
		allowedPaths = append(allowedPaths, landlock.DNS())
	}

	if allowCerts {
		allowedPaths = append(allowedPaths, landlock.Certs())
	}

	//remove duplicates
	var deduplicatedAllowedPaths []*landlock.Path
	for i, path1 := range allowedPaths {
		isDuplicate := false

		//search an equal path with a lower index.
		for _, path2 := range allowedPaths[:i] {
			if path1.Equal(path2) {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			deduplicatedAllowedPaths = append(deduplicatedAllowedPaths, path1)
		}
	}

	locker := landlock.New(deduplicatedAllowedPaths...)
	safety := landlock.OnlySupported //if running on Linux, require Landlock support.

	err := locker.Lock(safety)
	if err != nil {
		if runtime.GOOS == "linux" && errors.Is(err, landlock.ErrLandlockNotAvailable) {
			msg := landlock.ErrLandlockNotAvailable.Error() +
				": The kernel version is likely less than 5.13. In a few minor Inox versions the process will stop instead of showing this message."
			fmt.Println(msg)
		} else {
			panic(err)
		}
	}
}

func getLandlockPathForCommandPermission(perm core.CommandPermission, executablePaths map[string]struct{}, fls afs.Filesystem) *landlock.Path {
	switch cmdName := perm.CommandName.(type) {
	case core.Path:
		name := string(cmdName)
		if _, ok := executablePaths[name]; ok {
			return nil
		}

		//ignore if the executable does not exist.
		if _, err := fls.Stat(cmdName.UnderlyingString()); errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		executablePaths[name] = struct{}{}
		return landlock.File(name, "rx")
	case core.PathPattern:
		if cmdName.IsPrefixPattern() {
			return landlock.Dir(cmdName.Prefix(), "rx")
		} else {
			panic(core.ErrUnreachable)
		}
	case core.String:
		path, err := exec.LookPath(cmdName.UnderlyingString())
		if err != nil {
			//ignore if the executable does not exist.
			return nil
		}

		if _, ok := executablePaths[path]; ok {
			return nil
		}

		executablePaths[path] = struct{}{}

		return landlock.File(path, "rx")
	default:
		panic(core.ErrUnreachable)
	}

}

func updateFileAndDirLandlockPaths(
	perm core.FilesystemPermission,
	filePaths, dirPaths map[string]map[core.PermissionKind]struct{},
	fls afs.Filesystem,
) {
	var allowedPathString string

	dir := true

	switch entity := perm.Entity.(type) {
	case core.Path:
		allowedPathString = entity.UnderlyingString()

		if !entity.IsDirPath() {
			dir = false
		}

	case core.PathPattern:
		if entity.IsPrefixPattern() {
			allowedPathString = entity.Prefix()
		} else {
			//we try to find the longest path that contains all matched paths.

			segments := strings.Split(entity.UnderlyingString(), "/")
			lastIncludedSegmentIndex := -1

			//search the rightmost segment that has no special chars.
		loop:
			for segmentIndex, segment := range segments {
				runes := []rune(segment)

				for i, r := range runes {
					switch r {
					case '*', '?', '[':
						//ignore if escaped
						if i > 0 && utils.CountPrevBackslashes(runes, int32(i))%2 == 1 {
							continue
						}
						lastIncludedSegmentIndex = segmentIndex
						break loop
					}
				}
			}

			if lastIncludedSegmentIndex >= 0 {
				dir := strings.Join(segments[:lastIncludedSegmentIndex+1], "/")
				allowedPathString = dir
			} else if entity.IsDirGlobbingPattern() {
				allowedPathString = entity.UnderlyingString()
			} else {
				dir = false
				allowedPathString = entity.UnderlyingString()
			}
		}
	default:
		panic(core.ErrUnreachable)
	}

	//ignore non existing paths
	if _, err := fls.Stat(allowedPathString); errors.Is(err, fs.ErrNotExist) {
		return
	}

	if dir {
		opsAllowedOnDirTree, ok := dirPaths[allowedPathString]
		if !ok {
			opsAllowedOnDirTree = map[permkind.PermissionKind]struct{}{}
			dirPaths[allowedPathString] = opsAllowedOnDirTree
		}

		opsAllowedOnDirTree[perm.Kind_.Major()] = struct{}{}
	} else {
		opsAllowedOnFile, ok := filePaths[allowedPathString]
		if !ok {
			opsAllowedOnFile = map[permkind.PermissionKind]struct{}{}
			filePaths[allowedPathString] = opsAllowedOnFile
		}

		opsAllowedOnFile[perm.Kind_.Major()] = struct{}{}
	}

}
