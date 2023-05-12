package internal

import (
	"bufio"
	"fmt"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (s *HttpServer) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", s))
}

func (r *HttpRequest) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", r))
}

func (rw *HttpResponseWriter) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", rw))
}

func (r *HttpResponse) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {

	ctx := config.Context
	code := r.StatusCode(ctx)
	codeString := fmt.Sprintf("%d", code)
	if config.Colorize {
		if code < 400 {
			utils.Must(w.Write(config.Colors.SuccessColor))
		} else {
			utils.Must(w.Write(config.Colors.ErrorColor))
		}
		utils.Must(w.Write(utils.StringAsBytes(codeString)))
	}
	text := r.Status(ctx)
	text = strings.TrimSpace(strings.TrimPrefix(text, codeString))
	if text != "" {
		utils.PanicIfErr(w.WriteByte(' '))
		utils.Must(w.Write(utils.StringAsBytes(text)))
		utils.PanicIfErr(w.WriteByte(' '))
	}

	if config.Colorize {
		utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
	}
}

func (c *HttpClient) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", c))
}

func (evs *ServerSentEventSource) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", evs))
}
