package inoxsh_ns

import (
	"fmt"
	"log"
	"os"
)

func dbg(args ...any) {
	f, err := os.OpenFile(".debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		log.Panicln(err)
	}

	_, err = f.Write([]byte(fmt.Sprint(args...)))
	if err != nil {
		log.Panicln(err)
	}

	_, err = f.WriteString("\n")
	if err != nil {
		log.Panicln(err)
	}

	f.Close()
}
