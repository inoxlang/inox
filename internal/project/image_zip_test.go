package project

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestZipUnzipImage(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
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
			Id:                id,
			MaxFilesystemSize: 10 * MAX_UNCOMPRESSED_FS_SIZE,
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

		//Check Zip
		buf := bytes.NewBuffer(nil)

		err = img.Zip(ctx, buf)
		if !assert.NoError(t, err) {
			return
		}

		reader := bytes.NewReader(buf.Bytes())
		zipReader, err := zip.NewReader(reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Len(t, zipReader.File, 2)

		names := utils.MapSlice(zipReader.File, func(f *zip.File) string { return f.Name })

		assert.ElementsMatch(t, []string{inoxconsts.FS_DIR_SLASH_IN_IMG_ZIP, inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP}, names)

		//Check NewImageFromZip
		reader = bytes.NewReader(buf.Bytes())
		extractedImg, err := NewImageFromZip(ctx, reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, extractedImg.info.ProjectID, img.ProjectID())

		snapshot := extractedImg.FilesystemSnapshot()
		assert.Empty(t, snapshot.RootDirEntries())
	})

	t.Run("regular file at root level", func(t *testing.T) {
		project := createProject()

		fls := project.LiveFilesystem()
		utils.PanicIfErr(util.WriteFile(fls, "/x.ix", []byte("manifest {}"), 0600))

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		buf := bytes.NewBuffer(nil)
		err = img.Zip(ctx, buf)
		if !assert.NoError(t, err) {
			return
		}

		reader := bytes.NewReader(buf.Bytes())
		zipReader, err := zip.NewReader(reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Len(t, zipReader.File, 3)

		names := utils.MapSlice(zipReader.File, func(f *zip.File) string { return f.Name })

		assert.ElementsMatch(t, []string{inoxconsts.FS_DIR_SLASH_IN_IMG_ZIP, inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP, "fs/x.ix"}, names)

		//Check NewImageFromZip
		reader = bytes.NewReader(buf.Bytes())
		extractedImg, err := NewImageFromZip(ctx, reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, extractedImg.info.ProjectID, img.ProjectID())

		snapshot := extractedImg.FilesystemSnapshot()
		entries := snapshot.RootDirEntries()

		if !assert.NotEmpty(t, entries) {
			return
		}

		assert.Equal(t, "x.ix", entries[0])
	})

	t.Run("regular file in an arbitrary sub dir", func(t *testing.T) {
		project := createProject()

		fls := project.LiveFilesystem()
		utils.PanicIfErrAmong(
			fls.MkdirAll("/x", fs_ns.DEFAULT_DIR_FMODE),
			util.WriteFile(fls, "/x/x.ix", []byte("manifest {}"), 0600),
		)

		img, err := project.BaseImage()
		if !assert.NoError(t, err) {
			return
		}

		buf := bytes.NewBuffer(nil)
		err = img.Zip(ctx, buf)
		if !assert.NoError(t, err) {
			return
		}

		reader := bytes.NewReader(buf.Bytes())
		zipReader, err := zip.NewReader(reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Len(t, zipReader.File, 4)

		names := utils.MapSlice(zipReader.File, func(f *zip.File) string { return f.Name })

		assert.ElementsMatch(t, []string{inoxconsts.FS_DIR_SLASH_IN_IMG_ZIP, inoxconsts.IMAGE_INFO_FILE_IN_IMG_ZIP, "fs/x/", "fs/x/x.ix"}, names)

		//Check NewImageFromZip
		reader = bytes.NewReader(buf.Bytes())
		extractedImg, err := NewImageFromZip(ctx, reader, reader.Size())
		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, extractedImg.info.ProjectID, img.ProjectID())

		snapshot := extractedImg.FilesystemSnapshot()
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

	t.Run("large filesystem", func(t *testing.T) {
		//TODO
		// project := createProject()

		// fls := project.LiveFilesystem()
		// utils.PanicIfErr(util.WriteFile(fls, "/x.ix", bytes.Repeat([]byte{'a'}, MAX_UNCOMPRESSED_FS_SIZE+1), 0600))

		// img, err := project.BaseImage()
		// if !assert.NoError(t, err) {
		// 	return
		// }

		// buf := bytes.NewBuffer(nil)
		// err = img.Zip(ctx, buf)
		// if !assert.ErrorIs(t, err, ErrFilesystemSnapshotTooLarge) {
		// 	return
		// }

		//TODO: check NewImageFromZip by manually creating a zip.
	})
}
