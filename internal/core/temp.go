package core

import (
	"fmt"
	"math/rand"
	"time"

	afs "github.com/inoxlang/inox/internal/afs"
)

// CreateTempdir creates a directory with permissions o700 in /tmp.
func CreateTempdir(nameSecondPrefix string, fls afs.Filesystem) Path {

	path := Path(fmt.Sprintf("/tmp/inoxlang-%s-%d-%s", nameSecondPrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := fls.MkdirAll(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
}
