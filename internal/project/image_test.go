package project

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestBaseImage(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	reg := utils.Must(OpenRegistry(t.TempDir(), ctx))
	defer reg.Close(ctx)

	createProject := func() *Project {
		//create project
		params := CreateProjectParams{
			Name: "myproject",
		}
		id := utils.Must(reg.CreateProject(ctx, params))

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

	t.Run(".ix file at root level", func(t *testing.T) {
		project := createProject()

		fls := project.LiveFilesystem()
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

	t.Run(".ix file in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.LiveFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/xxx", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/xxx/x.ix", []byte("manifest {}"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		snapshot := img.FilesystemSnapshot()
		dir, err := snapshot.Metadata("/xxx")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, core.Path("/xxx/"), dir.AbsolutePath) {
			return
		}

		file, err := snapshot.Metadata("/xxx/x.ix")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.Path("/xxx/x.ix"), file.AbsolutePath)
	})
}
