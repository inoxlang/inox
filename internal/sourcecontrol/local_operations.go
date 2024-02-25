package sourcecontrol

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	ErrNoReference = errors.New("no reference")
	ErrEmptyCommit = errors.New("empty commit")
)

//Useful resources:
//- https://github.com/src-d/go-git/issues/604

// Stage stages a file or a directory.
func (r *GitRepository) Stage(absolutePath string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if absolutePath[0] != '/' {
		return fmt.Errorf("Stage() expects an absolute path")
	}

	workTree, err := r.inner.Worktree()

	if err != nil {
		return err
	}

	relativePath := toCleanRelativePath(absolutePath)

	_, err = workTree.Add(relativePath)
	return err
}

// Stage unstages a file or a directory.
func (r *GitRepository) Unstage(absolutePath string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if absolutePath[0] != '/' {
		return fmt.Errorf("Unstage() expects an absolute path")
	}

	relativePath := toCleanRelativePath(absolutePath)

	index, err := r.inner.Storer.Index()

	if err != nil {
		return err
	}

	//Get files to unstage.

	var files []string

	if absolutePath == "/" {
		return errors.New("unstaging all staged changes is not supported yet")
	}

	for _, e := range index.Entries {
		//If the unstage operation is on a dir we add all children and descendants.
		if filepath.Dir(e.Name) == relativePath {
			files = append(files, e.Name)
		}
	}

	if len(files) == 0 {
		files = append(files, relativePath)
	}

	// If there is no commit we remove directly from the index.

	_, err = r.inner.Head()

	if err != nil {
		for _, file := range files {
			index.Remove(file)
		}
		return r.inner.Storer.SetIndex(index)
	}

	// Other cases.

	return Restore(r.inner, &RestoreOptions{
		Staged: true,
		Files:  files,
	})
}

func (r *GitRepository) Commit(message string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	stagedChanges, err := r.getStagedChangesNoLock()

	if err != nil {
		return fmt.Errorf("failed to determine if there staged changes: %w", err)
	}

	if len(stagedChanges) == 0 {
		return ErrEmptyCommit
	}

	workTree, err := r.inner.Worktree()

	if err != nil {
		return err
	}

	_, err = workTree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "john doe",
			Email: "john.doe@example.com",
			When:  time.Now(),
		},
	})

	return err
}

func (r *GitRepository) GetUnstagedChanges() ([]Change, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	staged := false
	return r.getChangesNoLock(staged)
}

func (r *GitRepository) getStagedChangesNoLock() ([]Change, error) {
	staged := true
	return r.getChangesNoLock(staged)
}

func (r *GitRepository) GetStagedChanges() ([]Change, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.getStagedChangesNoLock()
}

func (r *GitRepository) LastDevCommit() (*object.Commit, error) {

	head, err := r.inner.Head()
	if err != nil {
		return nil, ErrNoReference
	}

	commit, err := r.inner.CommitObject(head.Hash())

	if err != nil {
		return nil, err
	}

	return commit, nil
}

// Log returns the commits reachable from $from (defaults to HEAD). All arguments are optional.
func (r *GitRepository) LogDevCommits(from plumbing.Hash, since, until *time.Time) ([]*object.Commit, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	iter, err := r.inner.Log(&git.LogOptions{
		Since: since,
		Until: until,
	})
	if err != nil {
		return nil, err
	}

	var commits []*object.Commit

	iter.ForEach(func(c *object.Commit) error {
		commits = append(commits, c)
		return nil
	})

	return commits, nil
}
