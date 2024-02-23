package project

import (
	"fmt"
	"path/filepath"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project/access"
)

func (p *Project) memberAndDevDir(ctx *core.Context, memberAuthToken string) (*access.Member, string, error) {
	id := access.MemberID(memberAuthToken)
	if err := id.Validate(); err != nil {
		return nil, "", err
	}

	member, err := p.AuthenticateMember(ctx, id)
	if err != nil {
		return nil, "", err
	}

	dir := filepath.Join(p.devDirOnOsFs, string(memberAuthToken))
	err = p.osFilesystem.MkdirAll(dir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return nil, "", err
	}

	return member, dir, nil
}

func (p *Project) DevDatabasesDirOnOsFs(ctx *core.Context, memberAuthToken string) (string, error) {
	_, memberDir, err := p.memberAndDevDir(ctx, memberAuthToken)

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

func (p *Project) DevFilesystem(ctx *core.Context, memberAuthToken string) (core.SnapshotableFilesystem, error) {

	//Get/create the directory that will store the developer's copy.

	member, memberDevDir, err := p.memberAndDevDir(ctx, memberAuthToken)
	if err != nil {
		return nil, err
	}

	memberName := member.Name()

	fsDir := filepath.Join(memberDevDir, FS_OS_DIR)
	err = p.osFilesystem.MkdirAll(fsDir, fs_ns.DEFAULT_DIR_FMODE)
	if err != nil {
		return nil, err
	}

	// Open the filesystem.

	developerCopyFS, err := fs_ns.OpenMetaFilesystem(ctx, p.osFilesystem, fs_ns.MetaFilesystemParams{
		Dir:            fsDir,
		MaxUsableSpace: p.maxFilesystemSize,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to open the copy (filesystem) of the project %s for member %s: %w", p.Id(), memberName, err)
	}

	closeFsBecauseOfError := true
	defer func() {
		if closeFsBecauseOfError {
			developerCopyFS.Close(ctx)
		}
	}()

	// If the filesystem is empty we copy the staging filesystem in it.

	rootEntries, err := developerCopyFS.ReadDir("/")

	if err != nil {
		return nil, fmt.Errorf("failed to read the copy (filesystem) of the project %s for member %s: %w", p.Id(), memberName, err)
	}

	if len(rootEntries) == 0 {
		snapshot, err := p.stagingFilesystem.TakeFilesystemSnapshot(core.FilesystemSnapshotConfig{
			GetContent:       core.NoContentCache,
			InclusionFilters: []core.PathPattern{"/..."},
		})

		if err != nil {
			return nil, fmt.Errorf("failed to take a snapshot of the project %s for member %s: %w", p.Id(), memberName, err)
		}

		err = snapshot.WriteTo(developerCopyFS, core.SnapshotWriteToFilesystem{Overwrite: false})

		if err != nil {
			return nil, fmt.Errorf("failed to create the copy of the project %s for member %s: %w", p.Id(), memberName, err)
		}
	}

	closeFsBecauseOfError = false
	return developerCopyFS, nil
}
