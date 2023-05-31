package s3_ns

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (b *Bucket) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(...)", b))
}

func (r *GetObjectResponse) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(...)", r))
}

func (r *PutObjectResponse) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(...)", r))
}

func (r *GetBucketPolicyResponse) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(%s)", r, r.s))
}

func (i *ObjectInfo) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	if i.Err != nil {
		if config.Colorize {
			utils.Must(w.Write(config.Colors.ErrorColor))
		}
		utils.Must(w.Write(utils.StringAsBytes(i.Err.Error())))

		if config.Colorize {
			utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
		}
	}

	if i.Key == "" || strings.HasSuffix(i.Key, "/") { //"folder"
		if config.Colorize {
			utils.Must(w.Write(config.Colors.Folder))
		}
		utils.Must(w.Write(utils.StringAsBytes(i.Key)))
		if config.Colorize {
			utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
		}

	} else {
		utils.Must(w.Write(utils.StringAsBytes(i.Key)))

		if config.Colorize {
			utils.Must(w.Write(config.Colors.DiscreteColor))
		}
		utils.PanicIfErr(w.WriteByte(' '))
		utils.Must(core.ByteCount(i.Size).Write(w, 1))
	}

	if config.Colorize {
		utils.Must(w.Write(config.Colors.DiscreteColor))
	}

	utils.PanicIfErr(w.WriteByte(' '))
	utils.Must(w.Write(utils.StringAsBytes(i.LastModified.Format(time.RFC3339))))

	if config.Colorize {
		utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
	}
}
