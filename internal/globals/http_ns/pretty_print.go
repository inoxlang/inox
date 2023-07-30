package http_ns

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
	text := utils.StripANSISequences(r.Status(ctx))

	text = strings.TrimSpace(strings.TrimPrefix(text, codeString))
	if text != "" {
		utils.PanicIfErr(w.WriteByte(' '))
		utils.Must(w.Write(utils.StringAsBytes(text)))
	}

	if config.Colorize {
		utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
		utils.Must(w.Write(config.Colors.DiscreteColor))
	}

	if r.wrapped.ContentLength >= 0 {
		utils.Must(w.Write(utils.StringAsBytes(" (")))
		length := core.ByteCount(r.wrapped.ContentLength)
		utils.Must(length.Write(w, 1))
		utils.PanicIfErr(w.WriteByte(')'))
	}

	contentType := r.wrapped.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "No Content-Type"
	}

	utils.PanicIfErr(w.WriteByte(' '))
	utils.Must(w.Write(utils.StringAsBytes(utils.StripANSISequences(contentType))))

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

func (v *ContentSecurityPolicy) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "ContentSecurityPolicy(%s)", v.String()))
}

func (p *HttpRequestPattern) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", p))
}
