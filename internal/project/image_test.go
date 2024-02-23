package project

import (
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestBaseImage(t *testing.T) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
	defer reg.Close(ctx)

	createProject := func() *Project {
		//create project
		params := CreateProjectParams{
			Name: "myproject",
		}
		id, _ := utils.Must2(reg.CreateProject(ctx, params))

		assert.NotEmpty(t, id)

		//open project
		project, err := reg.OpenProject(ctx, OpenProjectParams{
			Id: id,
		})

		if !assert.NoError(t, err) {
			t.FailNow()
		}

		return project
	}

	t.Run("empty filesystem", func(t *testing.T) {
		project := createProject()

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		assert.Empty(t, snapshot.RootDirEntries())
	})

	t.Run("regular file at root level", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/x.ix", []byte("manifest {}"), 0600))

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.NotEmpty(t, entries) {
			return
		}

		assert.Equal(t, "x.ix", entries[0])
	})

	t.Run("regular file in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/x", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/x/x.ix", []byte("manifest {}"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		dir, err := snapshot.Metadata("/x")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, core.Path("/x/"), dir.AbsolutePath) {
			return
		}

		file, err := snapshot.Metadata("/x/x.ix")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.Path("/x/x.ix"), file.AbsolutePath)
	})

	t.Run("dot file at root level", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/.file", []byte("hello"), 0600))

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.Empty(t, entries) {
			return
		}

		_, err = snapshot.Metadata("/.file")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("dot file in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/x", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/x/.file", []byte("hello"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.NotEmpty(t, entries) {
			return
		}

		assert.Equal(t, "x", entries[0])

		_, err = snapshot.Metadata("/x/.file")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("empty dot dir at root level", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/.dir", fs_ns.DEFAULT_DIR_FMODE),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.Empty(t, entries) {
			return
		}

		_, err = snapshot.Metadata("/.dir")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("non-empty dot dir at root level", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/.dir", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/.dir/script.ix", []byte("manifest {}"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.Empty(t, entries) {
			return
		}

		_, err = snapshot.Metadata("/.dir")
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = snapshot.Metadata("/.dir/script.ix")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("empty dot dir in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/x/.dir", fs_ns.DEFAULT_DIR_FMODE),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.NotEmpty(t, entries) {
			return
		}

		assert.Equal(t, "x", entries[0])

		_, err = snapshot.Metadata("/x/.dir")
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = snapshot.Metadata("/x/.dir/script.ix")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("non-empty dot dir in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.StagingFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/x/.dir", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/.dir/script.ix", []byte("manifest {}"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.NotEmpty(t, entries) {
			return
		}

		assert.Equal(t, "x", entries[0])

		_, err = snapshot.Metadata("/x/.dir")
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = snapshot.Metadata("/x/.dir/script.ix")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
