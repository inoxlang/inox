//go:build linux

package inoxprocess

import (
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/shoenig/go-landlock"
)

func restrictProcessAccess(grantedPerms, forbiddenPerms []core.Permission) {
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
			default:
				panic(core.ErrUnreachable)
			}
			allowedPaths = append(allowedPaths, allowedPath)
		case core.FilesystemPermission:
			var allowedPath *landlock.Path

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
				if entity.IsDirPath() {
					allowedPath = landlock.Dir(entity.UnderlyingString(), mode)
				} else {
					allowedPath = landlock.File(entity.UnderlyingString(), mode)
				}
			case core.PathPattern:
				mode := getMode(p.Kind())

				if entity.IsPrefixPattern() {
					allowedPath = landlock.Dir(entity.Prefix(), mode)
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
						allowedPath = landlock.Dir(dir, mode)
					} else if entity.IsDirGlobbingPattern() {
						allowedPath = landlock.Dir(entity.UnderlyingString(), mode)
					} else {
						allowedPath = landlock.File(entity.UnderlyingString(), mode)
					}
				}
			default:
				panic(core.ErrUnreachable)
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
