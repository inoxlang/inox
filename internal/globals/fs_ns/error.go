package fs_ns

import "fmt"

func fmtDirContainFiles(path string) string {
	return fmt.Sprintf("dir: %s contains files", path)
}
