package sourcecontrol

import (
	"os"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetChangesInEmptyRepository(t *testing.T) {

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

		err = repo.Stage("/file.txt")
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

func TestOperationsOnNonEmptyRepository(t *testing.T) {

	t.Run("commit file, modify file, stage file, unstage file", func(t *testing.T) {
		repo, fs := createEmptyRepo(t)
		util.WriteFile(fs, "/file.txt", []byte("initial content"), 0600)

		//Add the file and commit.

		utils.PanicIfErr(repo.Stage("/file.txt"))

		utils.PanicIfErr(repo.Commit("initial commit"))

		//Check unstaged changes.

		unstagedChanges, err := repo.GetUnstagedChanges()

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Empty(t, unstagedChanges) {
			return
		}

		//Modify the file.

		f := utils.Must(fs.OpenFile("/file.txt", os.O_WRONLY, 0600))
		f.Truncate(0)
		f.Write([]byte("content"))
		f.Close()

		//Check unstaged changes.

		unstagedChanges, err = repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		expectedUnstagedChanges := []Change{{AbsolutePath: "/file.txt", Code: git.Modified}}

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

		err = repo.Stage("/file.txt")
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

		expectedStagedChanges := []Change{{AbsolutePath: "/file.txt", Code: git.Modified}}

		if !assert.ElementsMatch(t, expectedStagedChanges, stagedChanges) {
			return
		}

		//Unstage the file.

		err = repo.Unstage("/file.txt")
		if !assert.NoError(t, err) {
			return
		}

		//Check unstaged changes.

		expectedUnstagedChanges = []Change{{AbsolutePath: "/file.txt", Code: git.Modified}}

		unstagedChanges, err = repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, expectedUnstagedChanges, unstagedChanges)

		//Check staged changes.

		stagedChanges, err = repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, stagedChanges)
	})

	t.Run("commit dir with files, modify a file, stage the dir, unstage the dir", func(t *testing.T) {
		repo, fs := createEmptyRepo(t)

		fs.MkdirAll("/dir", 0700)
		util.WriteFile(fs, "/dir/file.txt", []byte("initial content"), 0600)

		//Stage the dir and commit.

		utils.PanicIfErr(repo.Stage("/dir"))

		utils.PanicIfErr(repo.Commit("initial commit"))

		//Check unstaged changes.

		unstagedChanges, err := repo.GetUnstagedChanges()

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Empty(t, unstagedChanges) {
			return
		}

		//Modify the file.

		f := utils.Must(fs.OpenFile("/dir/file.txt", os.O_WRONLY, 0600))
		f.Truncate(0)
		f.Write([]byte("content"))
		f.Close()

		//Check unstaged changes.

		unstagedChanges, err = repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		expectedUnstagedChanges := []Change{{AbsolutePath: "/dir/file.txt", Code: git.Modified}}

		if !assert.ElementsMatch(t, expectedUnstagedChanges, unstagedChanges) {
			return
		}

		//Check staged changes.

		stagedChanges, err := repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, stagedChanges)

		//Stage the dir.

		err = repo.Stage("/dir")
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

		expectedStagedChanges := []Change{{AbsolutePath: "/dir/file.txt", Code: git.Modified}}

		if !assert.ElementsMatch(t, expectedStagedChanges, stagedChanges) {
			return
		}

		//Unstage the dir.

		err = repo.Unstage("/dir")
		if !assert.NoError(t, err) {
			return
		}

		//Check unstaged changes.

		expectedUnstagedChanges = []Change{{AbsolutePath: "/dir/file.txt", Code: git.Modified}}

		unstagedChanges, err = repo.GetUnstagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.ElementsMatch(t, expectedUnstagedChanges, unstagedChanges)

		//Check staged changes.

		stagedChanges, err = repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, stagedChanges)
	})

	t.Run("commit file, modify file, commit modification", func(t *testing.T) {
		repo, fs := createEmptyRepo(t)
		util.WriteFile(fs, "/file.txt", []byte("initial content"), 0600)

		//Add the file and commit.

		utils.PanicIfErr(repo.Stage("/file.txt"))

		utils.PanicIfErr(repo.Commit("initial commit"))

		//Modify the file.

		f := utils.Must(fs.OpenFile("/file.txt", os.O_WRONLY, 0600))
		f.Truncate(0)
		f.Write([]byte("content"))
		f.Close()

		//Stage the modification and commit.

		utils.PanicIfErr(repo.Stage("/file.txt"))

		utils.PanicIfErr(repo.Commit("modify file.txt"))

		//Check unstaged changes.

		unstagedChanges, err := repo.GetUnstagedChanges()

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Empty(t, unstagedChanges) {
			return
		}

		//Check staged changes.

		stagedChanges, err := repo.GetStagedChanges()
		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, stagedChanges)
	})
}

func createEmptyRepo(t *testing.T) (*GitRepository, billy.Filesystem) {
	worktreeFS := fs_ns.NewMemFilesystem(1_000_000)
	storage := memory.NewStorage()
	_, err := git.Init(storage, nil)

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	repo, err := git.Open(storage, worktreeFS)

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return WrapLower(repo), worktreeFS
}
