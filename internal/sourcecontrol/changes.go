package sourcecontrol

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project/imageconsts"
)

type Change struct {
	AbsolutePath string
	Code         git.StatusCode
}

// getChangesNoLock returns the staged changes if $staged is true. The unstaged changes otherwise.
func (r *GitRepository) getChangesNoLock(staged bool) ([]Change, error) {
	workTree, err := r.inner.Worktree()

	if err != nil {
		return nil, err
	}

	status, err := workTree.Status()

	if err != nil {
		return nil, err
	}

	index, err := r.inner.Storer.Index()

	if err != nil {
		return nil, err
	}

	_ = index

	var changes []Change

	core.WalkDir(workTree.Filesystem.(afs.Filesystem), "/", func(path core.Path, d fs.DirEntry, err error) error {
		if !d.Type().IsRegular() {
			return nil
		}

		for _, filter := range imageconsts.ABSOLUTE_EXCLUSION_FILTERS {
			if filter.Test(nil, path) {
				return nil
			}
		}

		absolutePath := string(path)
		relativePath := filepath.Clean(strings.TrimPrefix(string(path), "/"))
		fileStatus := status.File(relativePath)

		//TODO: support chunks.
		change := Change{
			AbsolutePath: absolutePath,
		}

		if staged {
			if fileStatus.Staging == git.Untracked {
				_, err := index.Entry(absolutePath)
				if err != nil {
					//Entry is not present, do not add the change.
					return nil
				}
			}
			change.Code = fileStatus.Staging
		} else { //unstaged
			if fileStatus.Worktree == git.Deleted && fileStatus.Staging != git.Deleted {
				//Do not add the change.
				return nil
			}

			_, err := index.Entry(absolutePath)
			if fileStatus.Worktree == git.Untracked && err == nil {
				//??? The file is tracked, do not add the change.
				return nil
			}
			change.Code = fileStatus.Worktree
		}

		changes = append(changes, change)
		return nil
	})

	return changes, nil
}
