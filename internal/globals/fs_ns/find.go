package fs_ns

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
)

func Find(ctx *core.Context, dir core.Path, filters ...core.Pattern) (*core.List, error) {
	if !dir.IsDirPath() {
		return nil, errors.New("find: first argument should be a directory path")
	}

	fls := ctx.GetFileSystem()

	//we check patterns & convert globbing patterns to absolute globbing path patterns.
	for i, filter := range filters {
		switch filt := filter.(type) {
		case core.StringPattern:
		case core.PathPattern:
			if !filt.IsGlobbingPattern() {
				return nil, errors.New("find: path filters should be globbing path patterns")
			}
			if !filt.IsAbsolute() {
				filt = core.PathPattern(fls.Join(string(dir), string(filt)))
				filters[i] = filt.ToAbs(fls)
			}
		default:
			return nil, fmt.Errorf("invalid pattern for filtering files: %s", core.Stringify(filt, ctx))
		}
	}

	var found []core.Serializable
	var paths []string

	//we first get matching paths
	for _, filter := range filters {
		switch filt := filter.(type) {
		case core.PathPattern:
			matches, err := glob(fls, string(filt))
			if err != nil {
				return nil, err
			}
			paths = append(paths, matches...)
		}
	}

	//we get the information for each matched file
	for _, pth := range paths {
		info, err := fls.Lstat(pth)
		if err != nil {
			return nil, err
		}
		found = append(found, makeFileInfo(info, pth, fls))
	}

	return core.NewWrappedValueList(found...), nil
}
