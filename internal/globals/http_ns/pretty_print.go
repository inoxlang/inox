package http_ns

import (
	"bufio"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

func (s *HttpsServer) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
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

	//write status
	{
		code := r.StatusCode(ctx)
		codeString := fmt.Sprintf("%d", code)

		if config.Colorize {
			utils.Must(w.Write(getStatusCodeColor(code, config.Colors)))
			utils.Must(w.Write(utils.StringAsBytes(codeString)))
		}
		text := utils.StripANSISequences(r.Status(ctx))

		text = strings.TrimSpace(strings.TrimPrefix(text, codeString))
		if text != "" {
			utils.PanicIfErr(w.WriteByte(' '))
			utils.Must(w.Write(utils.StringAsBytes(text)))
		}
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

func (s Status) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(getStatusCodeColor(s.code, config.Colors)))
	}
	utils.Must(w.WriteString(strconv.Itoa(int(s.code))))
	utils.PanicIfErr(w.WriteByte(' '))
	utils.Must(w.WriteString(s.reasonPhrase))
	if config.Colorize {
		utils.Must(w.Write(core.ANSI_RESET_SEQUENCE))
	}
}

func (c StatusCode) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	if config.Colorize {
		utils.Must(w.Write(getStatusCodeColor(c, config.Colors)))
	}
	utils.Must(w.WriteString(strconv.Itoa(int(c))))
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

func (csp *ContentSecurityPolicy) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "ContentSecurityPolicy(%s)", csp.String()))
}

func (p *HttpRequestPattern) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", p))
}

func getStatusCodeColor(code any, colors *prettyprint.PrettyPrintColors) []byte {
	val := reflect.ValueOf(code)

	var integer int64

	if val.CanInt() {
		integer = val.Int()
	} else {
		integer = int64(val.Uint())
	}

	if integer <= 399 {
		return colors.SuccessColor
	}
	if integer <= 499 {
		return colors.WarnColor
	}
	return colors.ErrorColor
}
