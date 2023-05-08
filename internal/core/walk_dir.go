package internal

import (
	"io/fs"

	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/utils"
)

type StatDirEntry struct {
	info fs.FileInfo
}

func NewStatDirEntry(info fs.FileInfo) *StatDirEntry {
	return &StatDirEntry{info}
}

func (d *StatDirEntry) Name() string               { return d.info.Name() }
func (d *StatDirEntry) IsDir() bool                { return d.info.IsDir() }
func (d *StatDirEntry) Type() fs.FileMode          { return d.info.Mode().Type() }
func (d *StatDirEntry) Info() (fs.FileInfo, error) { return d.info, nil }

// /adapted from stdlib path/filepath/path.go
func walkDir(fls afs.Filesystem, root string, fn fs.WalkDirFunc) error {
	info, err := fls.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = _walkDir(fls, root, &StatDirEntry{info}, fn)
	}
	if err == fs.SkipDir || err == fs.SkipAll {
		return nil
	}
	return err
}

func _walkDir(fls afs.Filesystem, path string, d fs.DirEntry, walkDirFn fs.WalkDirFunc) error {
	if err := walkDirFn(path, d, nil); err != nil || !d.IsDir() {
		if err == fs.SkipDir && d.IsDir() {
			// Successfully skipped directory.
			err = nil
		}
		return err
	}

	dirs, err := fls.ReadDir(path)
	if err != nil {
		// Second call, to report ReadDir error.
		err = walkDirFn(path, d, err)
		if err != nil {
			if err == fs.SkipDir && d.IsDir() {
				err = nil
			}
			return err
		}
	}

	entries := utils.MapSlice(dirs, func(i fs.FileInfo) fs.DirEntry {
		return &StatDirEntry{i}
	})

	for _, d1 := range entries {
		path1 := fls.Join(path, d1.Name())
		if err := _walkDir(fls, path1, d1, walkDirFn); err != nil {
			if err == fs.SkipDir {
				break
			}
			return err
		}
	}
	return nil
}
