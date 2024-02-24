package sourcecontrol

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestGetChanges(t *testing.T) {

	t.Run("untracked file, then with staged file", func(t *testing.T) {
		repo, fs := createEmptyRepo(t)
		util.WriteFile(fs, "/file.txt", nil, 0600)

		//Check unstaged changes.

		unstagedChanges, err := repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		expectedUnstagedChanges := []Change{{AbsolutePath: "/file.txt", Code: git.Untracked}}

		if !assert.ElementsMatch(t, expectedUnstagedChanges, unstagedChanges) {
			return
		}

		//Check staged changes.

		stagedChanges, err := repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, stagedChanges)

		//Stage the file.

		err = repo.Stage("file.txt")
		if !assert.NoError(t, err) {
			return
		}

		//Check unstaged changes.

		unstagedChanges, err = repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, unstagedChanges)

		//Check staged changes.

		stagedChanges, err = repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		expectedStagedChanges := []Change{{AbsolutePath: "/file.txt", Code: git.Added}}

		if !assert.ElementsMatch(t, expectedStagedChanges, stagedChanges) {
			return
		}
	})

}

func createEmptyRepo(t *testing.T) (*GitRepository, billy.Filesystem) {
	worktreeFS := fs_ns.NewMemFilesystem(1_000_000)
	storage := memory.NewStorage()
	_, err := git.Init(storage, worktreeFS)

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	repo, err := git.Open(storage, worktreeFS)

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return WrapLower(repo), worktreeFS
}
