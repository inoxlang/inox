package fs_ns

import (
	"errors"
	"os"
	"strings"

	fsutil "github.com/go-git/go-billy/v5/util"
	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

// Glob is the implementation of the fs.glob function, it calls the internal glob function.
func Glob(ctx *core.Context, patt core.PathPattern) []core.Path {

	if !patt.IsGlobbingPattern() {
		panic(errors.New("cannot call glob function on non-globbing pattern"))
	}

	fls := ctx.GetFileSystem()
	absPtt := patt.ToAbs(fls)

	res, err := glob(fls, string(absPtt))
	if err != nil {
		panic(err)
	}

	list := make([]core.Path, len(res))
	for i, e := range res {
		stat, err := fls.Stat(e)
		if err != nil {
			panic(err)
		}

		if e[0] != '/' {
			e = "./" + e
		}

		if stat.IsDir() {
			e += "/"
		}
		list[i] = core.Path(e)
	}
	return list
}

func glob(fls afs.Filesystem, absPattern string) (matches []string, e error) {
	if absPattern[0] != '/' {
		return nil, errors.New("only absolute pattern are supported")
	}

	type Entry struct {
		path  string
		index int
	}

	isDir := func(path string) (val bool, err error) {
		fi, err := fls.Stat(path)

		if err != nil {
			return false, err
		}

		return fi.IsDir(), nil
	}

	getSubDirs := func(path string) (dirs []string, err error) {
		if dir, err := isDir(path); err != nil || !dir {
			return nil, errors.New("not a directory " + path)
		}

		//TODO: add support to
		files, err := fls.ReadDir(path)

		if err != nil {
			return nil, err
		}

		for _, file := range files {
			path := fls.Join(path, file.Name())
			if dir, err := isDir(path); err == nil && dir {
				dirs = append(dirs, file.Name())
			}
		}
		return
	}

	if !strings.Contains(absPattern, "**") {
		return fsutil.Glob(fls, absPattern)
	}

	segments := strings.Split(absPattern, string(os.PathSeparator))
	workingEntries := []Entry{{path: "/", index: 0}}

	for len(workingEntries) > 0 {

		var temp []Entry
		for _, entry := range workingEntries {
			workingPath := entry.path
			idx := entry.index
			segment := segments[entry.index]

			if segment == "**" {
				// add all subdirectories and move yourself one step further
				// into pattern
				entry.index++

				subDirectories, err := getSubDirs(entry.path)

				if err != nil {
					return nil, err
				}

				for _, name := range subDirectories {
					path := fls.Join(workingPath, name)

					newEntry := Entry{
						path:  path,
						index: idx,
					}

					temp = append(temp, newEntry)
				}

			} else {
				// look at all results if we're at the end of the pattern, we found a match
				// else add it to a working entry
				path := fls.Join(workingPath, segment)
				results, err := fsutil.Glob(fls, path)

				if err != nil {
					return nil, err
				}

				for _, result := range results {
					if idx+1 < len(segments) {
						newEntry := Entry{
							path:  result,
							index: idx + 1,
						}

						temp = append(temp, newEntry)
					} else {
						matches = append(matches, result)
					}
				}
				// delete ourself regardless
				entry.index = len(segments)
			}

			// check whether current entry is still valid
			if entry.index < len(segments) {
				temp = append(temp, entry)
			}
		}

		workingEntries = temp
	}

	return
}
