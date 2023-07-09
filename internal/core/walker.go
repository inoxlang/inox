package core

import (
	"fmt"
	"io/fs"
	"strings"

	afs "github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/permkind"
)

var (
	_                           = []Walkable{Path("")}
	FS_TREE_DATA_ITEM_PROPNAMES = []string{"path", "path_rel_to_parent"}
)

type Walkable interface {
	Walker(*Context) (Walker, error)
}

type Walker interface {
	Iterator
	Prune(*Context)
	NodeMeta(*Context) WalkableNodeMeta
}

type WalkableNodeMeta struct {
	ancestors  []Value
	parentEdge Value
}

func NewWalkableNodeMeta(ancestors []Value, parentEdge Value) WalkableNodeMeta {
	return WalkableNodeMeta{
		ancestors:  ancestors,
		parentEdge: parentEdge,
	}
}

// GetWalkEntries walks a directory and returns all encountered entries and their paths in two 2D arrays.
// There is one slice for each directory, the first element (fs.DirEntry or path) of each slice is the directory.
// The others elements are the non-dir files inside the directory.
// For example if the walked directory only has a singike file inside it the result will be:
// entries: [ [<dir entry>, <file entry>] ]
// paths: [ [<dir path>, <file path> ] ]
func GetWalkEntries(fls afs.Filesystem, walkedDirPath Path) (entries [][]fs.DirEntry, paths [][]string) {

	walkDir(fls, string(walkedDirPath), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if d.IsDir() {
			if path[len(path)-1] != '/' {
				path += "/"
			}
		}

		if d.IsDir() {
			entries = append(entries, []fs.DirEntry{d})
			paths = append(paths, []string{string(path)})
		} else {
			entries[len(entries)-1] = append(entries[len(entries)-1], d)
			paths[len(paths)-1] = append(paths[len(paths)-1], string(path))
		}
		return nil
	})

	return entries, paths
}

func GetDirTreeData(fls afs.Filesystem, walkedDirPath Path) *UData {
	udata := &UData{
		Root: walkedDirPath,
	}

	baseDepth := strings.Count(string(walkedDirPath), "/")
	var dirStack []*UDataHiearchyEntry

	makeTreeDataItem := func(path, pathRelToOParent Path) *Record {
		return NewRecordFromKeyValLists(FS_TREE_DATA_ITEM_PROPNAMES, []Serializable{path, pathRelToOParent})
	}

	WalkDir(fls, walkedDirPath, func(path Path, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}

		relativePath := Path("./" + d.Name())

		if d.IsDir() {
			relativePath += "/"
			depth := strings.Count(string(path), "/") - baseDepth - 1
			if depth == 0 && d.Name() == string(walkedDirPath.Basename()) {
				return nil
			}
			dirStack = dirStack[:depth]
			value := makeTreeDataItem(path, relativePath)

			if len(dirStack) == 0 {
				udata.HiearchyEntries = append(udata.HiearchyEntries, UDataHiearchyEntry{Value: value})
				dirStack = []*UDataHiearchyEntry{&udata.HiearchyEntries[len(udata.HiearchyEntries)-1]}
			} else {
				parentDir := dirStack[len(dirStack)-1]
				parentDir.Children = append(parentDir.Children, UDataHiearchyEntry{Value: value})
				dirStack = append(dirStack, &parentDir.Children[len(parentDir.Children)-1])
			}
		} else {
			value := makeTreeDataItem(path, relativePath)

			depth := strings.Count(string(path), "/") - baseDepth
			dirStack = dirStack[:depth]

			if len(dirStack) == 0 {
				udata.HiearchyEntries = append(udata.HiearchyEntries, UDataHiearchyEntry{Value: value})
			} else {
				dirStack[len(dirStack)-1].Children = append(dirStack[len(dirStack)-1].Children, UDataHiearchyEntry{Value: value})
			}
		}
		return nil
	})

	return udata
}

// DirWalker is a Walker, it iterates over a list of known entries.
type DirWalker struct {
	NotClonableMixin

	dirIndex              int
	entryIndex            int
	entries               [][]fs.DirEntry
	paths                 [][]string
	addDotSlashPathPrefix bool
	walkedDirPath         Path
	skippedDirPath        string
	currentEntry          fs.DirEntry
	currentPath           string
	skipped               bool
	ancestors             []string // dir paths
}

// NewDirWalker walks a directory and creates a DirWalker with the entries.
func NewDirWalker(fls afs.Filesystem, walkedDirPath Path) *DirWalker {
	entries, paths := GetWalkEntries(fls, walkedDirPath)

	walker := &DirWalker{
		dirIndex:              0,
		entryIndex:            -1,
		entries:               entries,
		paths:                 paths,
		addDotSlashPathPrefix: walkedDirPath.IsRelative(),
		walkedDirPath:         walkedDirPath,
	}

	return walker
}

func WalkDir(fls afs.Filesystem, walkedDirPath Path, fn func(path Path, d fs.DirEntry, err error) error) {
	pathPrefix := ""

	if walkedDirPath.IsRelative() {
		pathPrefix = "./"
	}

	walkDir(fls, string(walkedDirPath), func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			if path[len(path)-1] != '/' {
				path += "/"
			}
		}
		return fn(Path(pathPrefix+path), d, err)
	})
}

func (it *DirWalker) HasNext(ctx *Context) bool {
	ok := it.dirIndex < len(it.entries)-1 || (it.dirIndex == len(it.entries)-1 && it.entryIndex < len(it.entries[it.dirIndex])-1)
	if !ok {
		return false
	}
	nextDirIndex := it.dirIndex
	nextEntryIndex := it.entryIndex + 1

	if nextEntryIndex >= len(it.entries[it.dirIndex]) {
		nextDirIndex++
		nextEntryIndex = 0
	}

	nextEntry := it.entries[nextDirIndex][nextEntryIndex]
	nextPath := it.paths[nextDirIndex][nextEntryIndex]
	if nextEntry.IsDir() && nextPath[len(nextPath)-1] != '/' {
		nextPath += "/"
	}

	if it.skippedDirPath != "" && strings.HasPrefix(nextPath, it.skippedDirPath) {
		it.dirIndex++
		it.entryIndex = 0
		it.skipped = true
		return it.HasNext(ctx)
	}
	return true
}

func (it *DirWalker) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	if !it.skipped {
		it.entryIndex++
		if it.entryIndex >= len(it.entries[it.dirIndex]) {
			it.dirIndex++
			it.entryIndex = 0
		}
	}

	it.currentEntry = it.entries[it.dirIndex][it.entryIndex]
	it.currentPath = it.paths[it.dirIndex][it.entryIndex]
	it.skipped = false
	it.skippedDirPath = ""
	return true
}

func (it *DirWalker) Prune(ctx *Context) {
	it.skippedDirPath = it.paths[it.dirIndex][0]

	if it.skippedDirPath != "" && it.skippedDirPath[len(it.skippedDirPath)-1] != '/' {
		it.skippedDirPath += "/"
	}
}

func (it *DirWalker) Key(*Context) Value {
	return Path(it.currentPath)
}

func (it *DirWalker) Value(*Context) Value {
	if it.currentEntry == nil {
		panic("no value")
	}
	return CreateDirEntry(it.currentPath, string(it.walkedDirPath), it.addDotSlashPathPrefix, it.currentEntry)
}

func (it *DirWalker) NodeMeta(*Context) WalkableNodeMeta {
	currentDirPath := it.paths[it.dirIndex][0]

	var ancestorPaths []Value

	for i := 0; i < it.dirIndex; i++ {
		ancestorPath := it.paths[i][0]

		// it's okay to use HasPrefix because the dir paths should always end with '/' so
		// we should never encounter the case of "/home/u" being a prefix of /home/user/dir
		if strings.HasPrefix(currentDirPath, ancestorPath) {
			ancestorPaths = append(ancestorPaths, Path(ancestorPath))
		}
	}

	return NewWalkableNodeMeta(ancestorPaths, Nil)
}

func (p Path) Walker(ctx *Context) (Walker, error) {
	if !p.IsDirPath() {
		return nil, fmt.Errorf("walks requires a directory path")
	}

	absPath, err := p.ToAbs(ctx.GetFileSystem())
	if err != nil {
		return nil, err
	}

	perm := FilesystemPermission{
		Kind_:  permkind.Read,
		Entity: PathPattern(string(absPath) + "..."),
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	return NewDirWalker(ctx.GetFileSystem(), p), nil
}
