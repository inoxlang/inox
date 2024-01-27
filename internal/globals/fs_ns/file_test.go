package fs_ns

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestFile(t *testing.T) {

	t.Run("missing write permission", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		err := f.write(ctx, core.String("hello"))
		assert.IsType(t, &core.NotAllowedError{}, err)
		assert.Equal(t, core.FilesystemPermission{
			Kind_:  permkind.WriteStream,
			Entity: utils.Must(pth.ToAbs(ctx.GetFileSystem())),
		}, err.(*core.NotAllowedError).Permission)
	})

	t.Run("rate limited", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Close()

		rate := int64(1000)

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
				core.FilesystemPermission{Kind_: permkind.WriteStream, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limit{
				{Name: FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: rate},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		//we first use all tokens
		data := core.NewImmutableByteSlice(bytes.Repeat([]byte{'x'}, int(rate)), "")
		err := f.write(ctx, data)
		assert.NoError(t, err)

		//we write again
		start := time.Now()
		err = f.write(ctx, data)
		assert.NoError(t, err)

		d := time.Since(start)

		//we check that we have been rate limited
		assert.Greater(t, d, time.Second)
	})

	t.Run("read should stop after context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		var pth = core.Path(filepath.Join(tmpDir, "file"))

		rate := int64(FS_READ_MIN_CHUNK_SIZE)
		fileSize := 10 * int(rate)

		osFile := utils.Must(os.Create(string(pth)))
		osFile.Write(bytes.Repeat([]byte{'x'}, fileSize))
		osFile.Close()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern(tmpDir + "/...")},
			},
			Limits: []core.Limit{
				{Name: FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: rate},
			},
			Filesystem: GetOsFilesystem(),
		})

		f := utils.Must(openExistingFile(ctx, pth, true))
		defer f.close(ctx)

		go func() {
			ctx.CancelGracefully()
		}()

		timeout := time.After(time.Second)

		//we read until context cancellation or after 1 second
		var successfullReads int
		var atLeastOneFailRead bool

	loop:
		for {
			select {
			case <-timeout:
				assert.Fail(t, "")
			default:
				slice, err := f.read(ctx)
				if len(slice.UnderlyingBytes()) != 0 {
					successfullReads += 1
				} else {
					assert.ErrorIs(t, err, context.Canceled)
					atLeastOneFailRead = true
					break loop
				}
			}
		}
		assert.True(t, atLeastOneFailRead)
		assert.LessOrEqual(t, successfullReads, 1)
	})
}
