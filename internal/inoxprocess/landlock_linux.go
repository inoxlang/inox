//go:build linux

package inoxprocess

import (
	"errors"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/shoenig/go-landlock"
)

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission, fls *fs_ns.OsFilesystem) {
	allowedPaths := []*landlock.Path{landlock.VMInfo(), landlock.Stdio()}
	var allowDNS, allowCerts bool

	for _, perm := range grantedPerms {
		switch p := perm.(type) {
		case core.DNSPermission:
			allowDNS = true
		case core.WebsocketPermission:
			allowCerts = true
		case core.HttpPermission:
			allowCerts = true
		case core.CommandPermission:
			var allowedPath *landlock.Path

			switch cmdName := p.CommandName.(type) {
			case core.Path:
				allowedPath = landlock.File(string(cmdName), "rx")
			case core.PathPattern:
				if cmdName.IsPrefixPattern() {
					allowedPath = landlock.Dir(cmdName.Prefix(), "rx")
				} else {
					panic(core.ErrUnreachable)
				}
			case core.Str:
				path, err := exec.LookPath(cmdName.UnderlyingString())
				if err != nil {
					panic(err)
				}
				allowedPath = landlock.File(path, "rx")
			default:
				panic(core.ErrUnreachable)
			}
			allowedPaths = append(allowedPaths, allowedPath)
		case core.FilesystemPermission:
			var allowedPath *landlock.Path
			var allowedPathString string

			getMode := func(kind permkind.PermissionKind) string {
				kind = kind.Major()
				switch kind {
				case permkind.Read:
					return "r"
				case permkind.Write, permkind.Delete:
					//TODO: improve

					return "wc"
				default:
					panic(core.ErrUnreachable)
				}
			}

			switch entity := p.Entity.(type) {
			case core.Path:
				mode := getMode(p.Kind())
				allowedPathString = entity.UnderlyingString()

				if entity.IsDirPath() {
					allowedPath = landlock.Dir(allowedPathString, mode)
				} else {
					allowedPath = landlock.File(allowedPathString, mode)
				}
			case core.PathPattern:
				mode := getMode(p.Kind())

				if entity.IsPrefixPattern() {
					allowedPathString = entity.Prefix()
					allowedPath = landlock.Dir(allowedPathString, mode)
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
						allowedPath = landlock.Dir(dir, mode)
					} else if entity.IsDirGlobbingPattern() {
						allowedPathString = entity.UnderlyingString()
						allowedPath = landlock.Dir(allowedPathString, mode)
					} else {
						allowedPathString = entity.UnderlyingString()
						allowedPath = landlock.File(allowedPathString, mode)
					}
				}
			default:
				panic(core.ErrUnreachable)
			}

			//ignore non existing paths
			if _, err := fls.Stat(allowedPathString); errors.Is(err, fs.ErrNotExist) {
				continue
			}

			allowedPaths = append(allowedPaths, allowedPath)
		}
	}

	if allowDNS {
		allowedPaths = append(allowedPaths, landlock.DNS())
	}

	if allowCerts {
		allowedPaths = append(allowedPaths, landlock.Certs())
	}

	locker := landlock.New(allowedPaths...)
	safety := landlock.OnlySupported //if running on Linux, require Landlock support.

	err := locker.Lock(safety)
	if err != nil {
		panic(err)
	}
}
