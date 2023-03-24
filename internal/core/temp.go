package internal

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

func CreateTempdir(nameSecondPrefix string) Path {

	path := Path(fmt.Sprintf("/tmp/inoxlang-%s-%d-%s", nameSecondPrefix, rand.Int(), time.Now().Format(time.RFC3339Nano)))

	if err := os.Mkdir(string(path), 0o700); err != nil {
		panic(err)
	}

	return path
}
