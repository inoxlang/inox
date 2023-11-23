package cloudflared

import (
	"fmt"
	"io"
	"os"
)

func RemoveCloudflaredDir(outW io.Writer) error {
	fmt.Fprintln(outW, "remove directory "+ROOT_CLOUDFLARED_DIR)
	return os.RemoveAll(ROOT_CLOUDFLARED_DIR)
}
