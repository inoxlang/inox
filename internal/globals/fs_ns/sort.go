package fs_ns

import "os"

type SortableFileInfo []os.FileInfo

func (a SortableFileInfo) Len() int           { return len(a) }
func (a SortableFileInfo) Less(i, j int) bool { return a[i].Name() < a[j].Name() }
func (a SortableFileInfo) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
