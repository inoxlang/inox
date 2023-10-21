package fs_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	fs_symbolic "github.com/inoxlang/inox/internal/globals/fs_ns/symbolic"
	"github.com/inoxlang/inox/internal/help"
)

func init() {
	core.RegisterDefaultPatternNamespace("fs", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"tree-data-item": core.NewInexactRecordPattern(map[string]core.Pattern{
				"path":               core.PATH_PATTERN,
				"path_rel_to_parent": core.PATH_PATTERN,
			}),
		},
	})

	//register limits
	core.LimRegistry.RegisterLimit(FS_READ_LIMIT_NAME, core.ByteRateLimit, FS_READ_MIN_CHUNK_SIZE)
	core.LimRegistry.RegisterLimit(FS_WRITE_LIMIT_NAME, core.ByteRateLimit, FS_WRITE_MIN_CHUNK_SIZE)
	core.LimRegistry.RegisterLimit(FS_NEW_FILE_RATE_LIMIT_NAME, core.SimpleRateLimit, 0)
	core.LimRegistry.RegisterLimit(FS_TOTAL_NEW_FILE_LIMIT_NAME, core.TotalLimit, 0)

	//register symbolic version of go functions
	core.RegisterSymbolicGoFunctions([]any{
		Mkfile, func(ctx *symbolic.Context, path *symbolic.Path, args ...symbolic.Value) *symbolic.Error {
			//ctx.SetSymbolicGoFunctionParameters(MKFILE_SYMB_PARAMS, MKFILE_ARG_NAMES)
			return nil
		},
		Mkdir, func(ctx *symbolic.Context, dirpath *symbolic.Path, content *symbolic.OptionalParam[*symbolic.Dictionary]) *symbolic.Error {
			ctx.SetSymbolicGoFunctionParameters(MKDIR_SYMB_PARAMS, MKDIR_ARG_NAMES)
			return nil
		},
		ReadFile, func(ctx *symbolic.Context, fpath *core.Path) (*symbolic.ByteSlice, *symbolic.Error) {
			ctx.SetSymbolicGoFunctionParameters(READFILE_SYMB_PARAMS, READFILE_ARG_NAMES)
			return symbolic.ANY_BYTE_SLICE, nil
		},
		Read, func(ctx *symbolic.Context, pth *symbolic.Path, args ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		ListFiles, func(ctx *symbolic.Context, pathOrPattern *symbolic.OptionalParam[symbolic.Value]) (*symbolic.List, *symbolic.Error) {
			ctx.SetSymbolicGoFunctionParameters(LISTFILES_SYMB_PARAMS, LISTFILES_ARG_NAMES)
			return symbolic.NewListOf(symbolic.ANY_FILEINFO), nil
		},
		Remove, func(ctx *symbolic.Context, args ...symbolic.Value) *symbolic.Error {
			return nil
		},
		Copy, func(ctx *symbolic.Context, args ...symbolic.Value) *symbolic.Error {
			return nil
		},
		Rename, func(ctx *symbolic.Context, old, new *symbolic.Path) *symbolic.Error {
			return nil
		},
		IsDir, func(ctx *symbolic.Context, pth *symbolic.Path) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		IsFile, func(ctx *symbolic.Context, pth *symbolic.Path) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		Exists, func(ctx *symbolic.Context, pth *symbolic.Path) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		Find, func(ctx *symbolic.Context, pth *symbolic.Path, filters ...symbolic.Pattern) (*symbolic.List, *symbolic.Error) {
			return &symbolic.List{}, nil
		},
		OpenExisting, func(ctx *symbolic.Context, args ...symbolic.Value) (*fs_symbolic.File, *symbolic.Error) {
			return &fs_symbolic.File{}, nil
		},
		Glob, func(ctx *symbolic.Context, patt ...*symbolic.PathPattern) *symbolic.List {
			return symbolic.NewListOf(&symbolic.Path{})
		},
		GetTreeData, func(ctx *symbolic.Context, pth *symbolic.Path) *symbolic.UData {
			return &symbolic.UData{}
		},
		NewMemFilesystemIL, func(ctx *symbolic.Context, maxTotalStorageSize *symbolic.ByteCount) *fs_symbolic.Filesystem {
			return fs_symbolic.ANY_FILESYSTEM
		},
		NewFilesystemSnapshot, func(ctx *symbolic.Context, args *symbolic.Object) *symbolic.FilesystemSnapshotIL {
			ctx.SetSymbolicGoFunctionParameters(NEW_FS_SNAPSHOT_SYMB_ARGS, NEW_FS_SNAPSHOT_SYMB_ARG_NAMES)
			return symbolic.ANY_FS_SNAPSHOT_IL
		},
	})

	help.RegisterHelpValues(map[string]any{
		"fs.mkfile":        Mkfile,
		"fs.mkdir":         Mkdir,
		"fs.read":          Read,
		"fs.ls":            ListFiles,
		"fs.rename":        Rename,
		"fs.mv":            Rename,
		"fs.cp":            Copy,
		"fs.exists":        Exists,
		"fs.isdir":         IsDir,
		"fs.isfile":        IsFile,
		"fs.remove":        Remove,
		"fs.glob":          Glob,
		"fs.find":          Find,
		"fs.get_tree_data": GetTreeData,
	})
}

func NewFsNamespace() *core.Namespace {
	return core.NewNamespace("fs", map[string]core.Value{
		"mkfile":             core.WrapGoFunction(Mkfile),
		"mkdir":              core.WrapGoFunction(Mkdir),
		"read_file":          core.WrapGoFunction(ReadFile),
		"read":               core.WrapGoFunction(Read),
		"ls":                 core.WrapGoFunction(ListFiles),
		"rm":                 core.WrapGoFunction(Remove),
		"remove":             core.WrapGoFunction(Remove),
		"cp":                 core.WrapGoFunction(Copy),
		"mv":                 core.WrapGoFunction(Rename),
		"rename":             core.WrapGoFunction(Rename),
		"isdir":              core.WrapGoFunction(IsDir),
		"isfile":             core.WrapGoFunction(IsFile),
		"find":               core.WrapGoFunction(Find),
		"exists":             core.WrapGoFunction(Exists),
		"open":               core.WrapGoFunction(OpenExisting),
		"glob":               core.WrapGoFunction(Glob),
		"get_tree_data":      core.WrapGoFunction(GetTreeData),
		"new_mem_filesystem": core.WrapGoFunction(NewMemFilesystemIL),
		"FsSnapshot":         core.WrapGoFunction(NewFilesystemSnapshot),
	})
}
