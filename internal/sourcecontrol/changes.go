package sourcecontrol

import (
	"fmt"
	"io/fs"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
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

	ref, err := r.inner.Head()
	var lastCommit *object.Commit

	var changes []Change

	seenFiles := map[string]struct{}{}

	if err == nil {
		lastCommit, err = r.inner.CommitObject(ref.Hash())
		if err != nil {
			return nil, fmt.Errorf("failed to get commit pointed by HEAD: %w", err)
		}

		files, err := lastCommit.Files()

		if err != nil {
			return nil, fmt.Errorf("failed to get files in commit pointed by HEAD: %w", err)
		}

		files.ForEach(func(f *object.File) error {

			relativePath := toCleanRelativePath(f.Name)

			seenFiles[relativePath] = struct{}{}

			change, ok := r.getFileChange(relativePath, staged, status, index, lastCommit)
			if ok {
				changes = append(changes, change)
			}

			return nil
		})
	}

	core.WalkDir(workTree.Filesystem.(afs.Filesystem), "/", func(path core.Path, d fs.DirEntry, err error) error {
		if !d.Type().IsRegular() {
			return nil
		}

		relativePath := toCleanRelativePath(string(path))
		if _, ok := seenFiles[relativePath]; ok {
			return nil
		}

		seenFiles[relativePath] = struct{}{}

		change, ok := r.getFileChange(relativePath, staged, status, index, lastCommit)
		if ok {
			changes = append(changes, change)
		}

		return nil
	})

	return changes, nil
}

func (r *GitRepository) getFileChange(relativePath string, staged bool, status git.Status, index *index.Index, lastCommit *object.Commit) (Change, bool) {

	for _, filter := range imageconsts.RELATIVE_EXCLUSION_FILTERS {
		if filter.Test(nil, core.Path(relativePath)) {
			return Change{}, false
		}
	}

	fileStatus := status.File(relativePath)

	_, err := index.Entry(relativePath)
	inIndex := err == nil

	//TODO: support chunks.
	change := Change{}

	if staged {
		if fileStatus.Staging == git.Untracked {
			if !inIndex {
				//Entry is not present, do not add the change.
				return Change{}, false
			}
		}
		if fileStatus.Staging == git.Unmodified {
			return Change{}, false
		}

		if fileStatus.Staging == git.Untracked /* https://github.com/go-git/go-git/issues/119 */ && inIndex {
			return Change{}, false
		}

		change.Code = fileStatus.Staging
	} else { //unstaged

		if fileStatus.Worktree == git.Deleted && fileStatus.Staging != git.Deleted {
			//Do not add the change.
			return Change{}, false
		}

		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging != git.Unmodified {
			//Do not add the change.
			return Change{}, false
		}

		if fileStatus.Worktree == git.Untracked && err == nil {
			//??? The file is tracked, do not add the change.
			return Change{}, false
		}
		change.Code = fileStatus.Worktree
	}

	change.AbsolutePath = toCleanAbsolutePath(relativePath)

	return change, true
}
