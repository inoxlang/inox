package fsutils

import (
	"io/fs"
	"os"
)

// WriteFileSync is the same function as os.WriteFile but it syncs the file before returning.
func WriteFileSync(name string, data []byte, perm fs.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Sync()
	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}
	return err
}
