package sourcecontrol

import (
	"path/filepath"
)

//Useful resources:
//- https://github.com/src-d/go-git/issues/604

func (r *GitRepository) StageFile(relativePath string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	relativePath = filepath.Clean(relativePath)
	workTree, err := r.inner.Worktree()

	if err != nil {
		return err
	}

	_, err = workTree.Add(relativePath)
	return err
}

func (r *GitRepository) GetUnstagedChanges() ([]Change, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	staged := false
	return r.getChangesNoLock(staged)
}

func (r *GitRepository) GetStagedChanges() ([]Change, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	staged := true
	return r.getChangesNoLock(staged)
}
