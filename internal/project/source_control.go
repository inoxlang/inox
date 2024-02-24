package project

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

const (
	LARGE_GIT_OBJECT_THRESHOLD = 10_000_000
)

func (c *developerCopy) openRepositoryNoLock(gitStorageFs *fs_ns.MetaFilesystem) error {
	if c.workingFs == nil {
		panic(errors.New("working tree filesystem should be open"))
	}
	if c.gitStorageFs == nil {
		panic(errors.New("git storage filesystem should be open"))
	}

	gitStorage := filesystem.NewStorageWithOptions(gitStorageFs, cache.NewObjectLRUDefault(), filesystem.Options{
		ExclusiveAccess:      true,
		LargeObjectThreshold: LARGE_GIT_OBJECT_THRESHOLD,
	})

	repo, err := git.Open(gitStorage, c.workingFs)

	if errors.Is(err, git.ErrRepositoryNotExists) {
		//Initialize the repository.

		_, err = git.Init(gitStorage, nil /*passing c.workinFs here causes the files to be deleted.*/)

		if err != nil {
			return fmt.Errorf("failed to initialize git repository on the project server: %w", err)
		}

		repo, err = git.Open(gitStorage, c.workingFs)

		if err != nil {
			return fmt.Errorf("failed to open git repository on the project server: %w", err)
		}

	} else if err != nil {
		return fmt.Errorf("failed to open git repository on project server: %w", err)
	}

	c.repository = repo
	return nil
}
