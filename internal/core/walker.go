package core

import (
	"fmt"
	"io/fs"
	"strings"

	afs "github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
)

var (
	_                           = []Walkable{Path(""), (*Treedata)(nil)}
	FS_TREE_DATA_ITEM_PROPNAMES = []string{"path", "path_rel_to_parent"}
)

// A Walkable is value that can be walked using a walker.
type Walkable interface {

	//Walker should return a new walker that, when possible, should be not affected by mutations of the walked value.
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

func GetDirTreeData(fls afs.Filesystem, walkedDirPath Path) *Treedata {
	treedata := &Treedata{
		Root: walkedDirPath,
	}

	baseDepth := strings.Count(string(walkedDirPath), "/")
	var dirStack []*TreedataHiearchyEntry

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
				treedata.HiearchyEntries = append(treedata.HiearchyEntries, TreedataHiearchyEntry{Value: value})
				dirStack = []*TreedataHiearchyEntry{&treedata.HiearchyEntries[len(treedata.HiearchyEntries)-1]}
			} else {
				parentDir := dirStack[len(dirStack)-1]
				parentDir.Children = append(parentDir.Children, TreedataHiearchyEntry{Value: value})
				dirStack = append(dirStack, &parentDir.Children[len(parentDir.Children)-1])
			}
		} else {
			value := makeTreeDataItem(path, relativePath)

			depth := strings.Count(string(path), "/") - baseDepth
			dirStack = dirStack[:depth]

			if len(dirStack) == 0 {
				treedata.HiearchyEntries = append(treedata.HiearchyEntries, TreedataHiearchyEntry{Value: value})
			} else {
				dirStack[len(dirStack)-1].Children = append(dirStack[len(dirStack)-1].Children, TreedataHiearchyEntry{Value: value})
			}
		}
		return nil
	})

	return treedata
}

// DirWalker is a Walker, it iterates over a list of known entries.
type DirWalker struct {
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

type TreedataWalker struct {
	chain      []TreedataHiearchyEntry
	indexChain []int
	nextIndex  int
}

func (d *Treedata) Walker(*Context) (Walker, error) {
	rootPseudoEntry := TreedataHiearchyEntry{
		Value:    d.Root,
		Children: d.HiearchyEntries,
	}

	return &TreedataWalker{
		chain:      []TreedataHiearchyEntry{rootPseudoEntry},
		indexChain: []int{0},
	}, nil
}

func (it *TreedataWalker) HasNext(ctx *Context) bool {
	if it.nextIndex == 0 {
		return true
	}

	currentEntry := it.chain[len(it.chain)-1]

	indexChain := it.indexChain
	chain := it.chain

	if len(currentEntry.Children) == 0 {
		chain = chain[:len(chain)-1]
	} else {
		return true
	}

	var childIndex int

	for len(chain) > 0 {
		parentEntry := chain[len(chain)-1]
		childIndex = indexChain[len(indexChain)-1]

		//last children
		if childIndex == len(parentEntry.Children)-1 {
			indexChain = indexChain[:len(indexChain)-1]
			chain = chain[:len(chain)-1]

			continue
		}

		return true
	}

	return false
}

func (it *TreedataWalker) Next(ctx *Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	if it.nextIndex == 0 {
		it.nextIndex++
		return true
	}

	currentEntry := it.chain[len(it.chain)-1]

	if len(currentEntry.Children) == 0 { //pop current entry
		it.chain = it.chain[:len(it.chain)-1]
	} else { //add first child
		it.chain = append(it.chain, currentEntry.Children[0])
		it.indexChain = append(it.indexChain, 0)
		it.nextIndex++
		return true
	}

	var childIndex int

	for len(it.chain) > 0 {
		parentEntry := it.chain[len(it.chain)-1]
		childIndex = it.indexChain[len(it.indexChain)-1]

		//last children
		if childIndex == len(parentEntry.Children)-1 {
			it.indexChain = it.indexChain[:len(it.indexChain)-1]
			it.chain = it.chain[:len(it.chain)-1]

			continue
		}

		childIndex++

		it.indexChain[len(it.indexChain)-1] = childIndex
		it.chain[len(it.chain)-1] = parentEntry.Children[childIndex]
		it.nextIndex++
		return true
	}

	return false
}

func (it *TreedataWalker) Prune(ctx *Context) {

}

func (it *TreedataWalker) Key(*Context) Value {
	return Int(it.nextIndex - 1)
}

func (it *TreedataWalker) Value(*Context) Value {
	return it.chain[len(it.chain)-1].Value
}

func (it *TreedataWalker) NodeMeta(*Context) WalkableNodeMeta {
	if it.nextIndex == 1 {
		return NewWalkableNodeMeta(nil, Nil)
	}
	return WalkableNodeMeta{}
}
