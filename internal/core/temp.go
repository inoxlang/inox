package core

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

// CreateTempdir creates a directory with permissions o700 in /tmp.
func CreateTempdir(nameSecondPrefix string) Path {

	path := Path(fmt.Sprintf("/tmp/inoxlang-%s-%d-%s", nameSecondPrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := os.MkdirAll(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
}
