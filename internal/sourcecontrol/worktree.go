package sourcecontrol

//This file contains pieces of the original worktree implementation https://github.com/go-git/go-git/blob/master/worktree.go (Apache 2.0 License)
//and includes:
// - The pull request https://github.com/go-git/go-git/pull/493 by Ben Talbot (https://github.com/ben-tbotlabs). Thank you Ben.
//The original worktree implementation is still used for most operations.
//Other pull requests are custom changes made by applied in the future.

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	mindex "github.com/go-git/go-git/v5/utils/merkletrie/index"
	"github.com/go-git/go-git/v5/utils/merkletrie/noder"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

var (
	ErrWorktreeNotClean                 = errors.New("worktree is not clean")
	ErrUnstagedChanges                  = errors.New("worktree contains unstaged changes")
	ErrNonFastForwardUpdate             = errors.New("non-fast-forward update")
	ErrRestoreWorktreeeOnlyNotSupported = errors.New("worktree only is not supported")
)

// Worktree represents a git worktree.
type Worktree struct {
	// Filesystem underlying filesystem.
	Filesystem billy.Filesystem

	r *git.Repository
}

// RestoreOptions describes how a restore should be performed.
type RestoreOptions struct {
	// Marks to restore the content in the index
	Staged bool
	// Marks to restore the content of the working tree
	Worktree bool
	// List of file paths that will be restored
	Files []string
}

// Validate validates the fields and sets the default values.
func (o *RestoreOptions) Validate() error {
	if len(o.Files) == 0 {
		return ErrNoRestorePaths
	}

	return nil
}

// Restore specified files in the working tree or stage with contents from
// a restore source. If a path is tracked but does not exist in the restore,
// source, it will be removed to match the source.
//
// If Staged and Worktree are true, then the restore source will be the index.
// If only Staged is true, then the restore source will be HEAD.
// If only Worktree is true or neither Staged nor Worktree are true, will
// result in ErrRestoreWorktreeeOnlyNotSupported because restoring the working
// tree while leaving the stage untouched is not currently supported
//
// Restore with no files specified will return ErrNoRestorePaths
func Restore(r *git.Repository, o *RestoreOptions) error {

	trueWorktree, err := r.Worktree()
	if err != nil {
		return err
	}

	w := Worktree{
		r:          r,
		Filesystem: trueWorktree.Filesystem,
	}

	if err := o.Validate(); err != nil {
		return err
	}

	if o.Worktree && o.Staged {
		// If we are doing both Worktree and Staging then it is a hard reset
		opts := &ResetOptions{
			Mode:  git.HardReset,
			Files: o.Files,
		}
		return w.Reset(opts)
	} else if o.Staged {
		// If we are doing just staging then it is a mixed reset
		opts := &ResetOptions{
			Mode:  git.MixedReset,
			Files: o.Files,
		}
		return w.Reset(opts)
	} else {
		return ErrRestoreWorktreeeOnlyNotSupported
	}
}

// ResetOptions describes how a reset operation should be performed.
type ResetOptions struct {
	// Commit, if commit is present set the current branch head (HEAD) to it.
	Commit plumbing.Hash
	// Mode, form resets the current branch head to Commit and possibly updates
	// the index (resetting it to the tree of Commit) and the working tree
	// depending on Mode. If empty MixedReset is used.
	Mode git.ResetMode
	// Files, if not empty will constrain the reseting the index to only files
	// specified in this list
	Files []string
}

var (
	ErrNoRestorePaths = errors.New("you must specify path(s) to restore")
)

// Validate validates the fields and sets the default values.
func (o *ResetOptions) Validate(r *git.Repository) error {
	if o.Commit == plumbing.ZeroHash {
		ref, err := r.Head()
		if err != nil {
			return err
		}

		o.Commit = ref.Hash()
	}

	return nil
}

func (w *Worktree) ResetSparsely(opts *ResetOptions, dirs []string) error {
	if err := opts.Validate(w.r); err != nil {
		return err
	}

	if opts.Mode == git.MergeReset {
		return fmt.Errorf("merge resets are not supported")
	}

	if err := w.setHEADCommit(opts.Commit); err != nil {
		return err
	}

	if opts.Mode == git.SoftReset {
		return nil
	}

	t, err := getTreeFromCommitHash(w.r, opts.Commit)
	if err != nil {
		return err
	}

	if opts.Mode == git.MixedReset || opts.Mode == git.MergeReset || opts.Mode == git.HardReset {
		if err := w.resetIndex(t, dirs, opts.Files); err != nil {
			return err
		}
	}

	if opts.Mode == git.MergeReset || opts.Mode == git.HardReset {
		return fmt.Errorf("merge and hard resets are not supported")
	}

	return nil
}

// Reset the worktree to a specified state.
func (w *Worktree) Reset(opts *ResetOptions) error {
	return w.ResetSparsely(opts, nil)
}

func (w *Worktree) resetIndex(t *object.Tree, dirs []string, files []string) error {
	idx, err := w.r.Storer.Index()
	if len(dirs) > 0 {
		idx.SkipUnless(dirs)
	}

	if err != nil {
		return err
	}
	b := newIndexBuilder(idx)

	changes, err := w.diffTreeWithStaging(t, true)
	if err != nil {
		return err
	}

	for _, ch := range changes {
		a, err := ch.Action()
		if err != nil {
			return err
		}

		var name string
		var e *object.TreeEntry

		switch a {
		case merkletrie.Modify, merkletrie.Insert:
			name = ch.To.String()
			e, err = t.FindEntry(name)
			if err != nil {
				return err
			}
		case merkletrie.Delete:
			name = ch.From.String()
		}

		if len(files) > 0 {
			contains := slices.Contains(files, name)
			if !contains {
				continue
			}
		}

		b.Remove(name)
		if e == nil {
			continue
		}

		b.Add(&index.Entry{
			Name: name,
			Hash: e.Hash,
			Mode: e.Mode,
		})

	}

	b.Write(idx)
	return w.r.Storer.SetIndex(idx)
}

func (w *Worktree) diffTreeWithStaging(t *object.Tree, reverse bool) (merkletrie.Changes, error) {
	var from noder.Noder
	if t != nil {
		from = object.NewTreeRootNode(t)
	}

	idx, err := w.r.Storer.Index()
	if err != nil {
		return nil, err
	}

	to := mindex.NewRootNode(idx)

	if reverse {
		return merkletrie.DiffTree(to, from, diffTreeIsEquals)
	}

	return merkletrie.DiffTree(from, to, diffTreeIsEquals)
}

func (w *Worktree) setHEADCommit(commit plumbing.Hash) error {
	head, err := w.r.Reference(plumbing.HEAD, false)
	if err != nil {
		return err
	}

	if head.Type() == plumbing.HashReference {
		head = plumbing.NewHashReference(plumbing.HEAD, commit)
		return w.r.Storer.SetReference(head)
	}

	branch, err := w.r.Reference(head.Target(), false)
	if err != nil {
		return err
	}

	if !branch.Name().IsBranch() {
		return fmt.Errorf("invalid HEAD target should be a branch, found %s", branch.Type())
	}

	branch = plumbing.NewHashReference(branch.Name(), commit)
	return w.r.Storer.SetReference(branch)
}

func getTreeFromCommitHash(repository *git.Repository, commit plumbing.Hash) (*object.Tree, error) {
	c, err := repository.CommitObject(commit)
	if err != nil {
		return nil, err
	}

	return c.Tree()
}

var emptyNoderHash = make([]byte, 24)

// diffTreeIsEquals is a implementation of noder.Equals, used to compare
// noder.Noder, it compare the content and the length of the hashes.
//
// Since some of the noder.Noder implementations doesn't compute a hash for
// some directories, if any of the hashes is a 24-byte slice of zero values
// the comparison is not done and the hashes are take as different.
func diffTreeIsEquals(a, b noder.Hasher) bool {
	hashA := a.Hash()
	hashB := b.Hash()

	if bytes.Equal(hashA, emptyNoderHash) || bytes.Equal(hashB, emptyNoderHash) {
		return false
	}

	return bytes.Equal(hashA, hashB)
}

type indexBuilder struct {
	entries map[string]*index.Entry
}

func newIndexBuilder(idx *index.Index) *indexBuilder {
	entries := make(map[string]*index.Entry, len(idx.Entries))
	for _, e := range idx.Entries {
		entries[e.Name] = e
	}
	return &indexBuilder{
		entries: entries,
	}
}

func (b *indexBuilder) Write(idx *index.Index) {
	idx.Entries = idx.Entries[:0]
	for _, e := range b.entries {
		idx.Entries = append(idx.Entries, e)
	}
}

func (b *indexBuilder) Add(e *index.Entry) {
	b.entries[e.Name] = e
}

func (b *indexBuilder) Remove(name string) {
	delete(b.entries, filepath.ToSlash(name))
}
