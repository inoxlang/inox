package internal

import (
	"fmt"
	"math/rand"
	"time"

	afs "github.com/go-git/go-billy/v5"
)

func CreateTempdir(nameSecondPrefix string, fls afs.Filesystem) Path {

	path := Path(fmt.Sprintf("/tmp/inoxlang-%s-%d-%s", nameSecondPrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := fls.MkdirAll(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
}
