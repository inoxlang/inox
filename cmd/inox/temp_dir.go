package main

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

func CreateTempDir() (processTempDir core.Path, processTempDirPerms []core.Permission, removeDir func()) {
	processTempDir = fs_ns.GetCreateProcessTempDir()
	processTempDirPrefix := core.AppendTrailingSlashIfNotPresent(core.PathPattern(processTempDir)) + "..."

	processTempDirPerms = []core.Permission{
		core.FilesystemPermission{Kind_: permbase.Read, Entity: processTempDirPrefix},
		core.FilesystemPermission{Kind_: permbase.Write, Entity: processTempDirPrefix},
		core.FilesystemPermission{Kind_: permbase.Delete, Entity: processTempDirPrefix},
	}
	removeDir = func() {
		fs_ns.GetOsFilesystem().RemoveAll(processTempDir.UnderlyingString())
	}

	return
}
