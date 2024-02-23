package project

import (
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project/access"
)

func (p *Project) MemberDevDir(ctx *core.Context, memberAuthToken string) (string, error) {
	id := access.MemberID(memberAuthToken)
	if err := id.Validate(); err != nil {
		return "", err
	}

	_, err := p.AuthenticateMember(ctx, id)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(p.devDirOnOsFs, string(memberAuthToken))
	err = p.osFilesystem.MkdirAll(dir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", err
	}

	return dir, nil
}

func (p *Project) DevDatabasesDirOnOsFs(ctx *core.Context, memberAuthToken string) (string, error) {
	memberDir, err := p.MemberDevDir(ctx, memberAuthToken)

	if err != nil {
		return "", err
	}

	dir := filepath.Join(memberDir, DEV_DATABASES_OS_DIR)
	err = p.osFilesystem.MkdirAll(dir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return "", err
	}

	return dir, nil
}

func (p *Project) DevFilesystem(id access.MemberID) core.SnapshotableFilesystem {
	return nil
}
