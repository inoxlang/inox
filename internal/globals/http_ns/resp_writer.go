package http_ns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aohorodnyk/mimeheader"
	core "github.com/inoxlang/inox/internal/core"

	"github.com/rs/zerolog"
)

const (
	REQUEST_ID_LOG_FIELD_NAME = "reqID"
)

var (
	ErrNotAcceptedContentType       = errors.New("not accepted content type")
	ErrCannotMutateFinishedResponse = errors.New("cannot mutate finished response")
	ErrStatusAlreadySet             = errors.New("status already set")

	RESP_WRITER_PROPNAMES = []string{
		"write_text", "write_binary", "write_html", "write_json", "write_ixon", "set_cookie", "write_status", "write_error",
		"add_header",
	}
)

type HttpResponseWriter struct {
	core.NoReprMixin
	core.NotClonableMixin

	request      *HttpRequest
	acceptHeader mimeheader.AcceptHeader
	rw           http.ResponseWriter

	status   int //do not use, call Status() to get the status
	finished bool
	logger   zerolog.Logger
}

func NewResponseWriter(req *HttpRequest, rw http.ResponseWriter, serverLogger zerolog.Logger) *HttpResponseWriter {
	requestLogger := serverLogger.With().Str(REQUEST_ID_LOG_FIELD_NAME, req.ULIDString).Logger()

	//log request
	event := requestLogger.Info().
		Str("method", string(req.Method)).
		Str("path", string(req.Path))

	query := req.request.URL.RawQuery
	if query != "" {
		event.Str("query", query)
	}
	event.Send()

	return &HttpResponseWriter{
		acceptHeader: req.ParsedAcceptHeader,
		rw:           rw,
		request:      req,
		status:       -1,
		logger:       requestLogger,
	}
}

func (rw *HttpResponseWriter) GetGoMethod(name string) (*core.GoFunction, bool) {
	rw.assertIsNotFinished()

	switch name {
	case "write_text":
		return core.WrapGoMethod(rw.WritePlainText), true
	case "write_binary":
		return core.WrapGoMethod(rw.WriteBinary), true
	case "write_html":
		return core.WrapGoMethod(rw.WriteHTML), true
	case "write_json":
		return core.WrapGoMethod(rw.WriteJSON), true
	case "write_ixon":
		return core.WrapGoMethod(rw.WriteIXON), true
	case "set_cookie":
		return core.WrapGoMethod(rw.SetCookie), true
	case "write_status":
		return core.WrapGoMethod(rw.WriteStatus), true
	case "write_error":
		return core.WrapGoMethod(rw.WriteError), true
	case "add_header":
		return core.WrapGoMethod(rw.AddHeader), true
	default:
		return nil, false
	}
}

func (rw *HttpResponseWriter) Prop(ctx *core.Context, name string) core.Value {
	rw.assertIsNotFinished()

	method, ok := rw.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, rw))
	}
	return method
}

func (*HttpResponseWriter) SetProp(ctx *core.Context, name string, value core.Value) error {

	return core.ErrCannotSetProp
}

func (rw *HttpResponseWriter) PropertyNames(ctx *core.Context) []string {
	rw.assertIsNotFinished()
	return RESP_WRITER_PROPNAMES
}

func (rw *HttpResponseWriter) WritePlainText(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.PLAIN_TEXT_CTYPE) {
		return 0, fmt.Errorf("cannot write plain text: %w", ErrNotAcceptedContentType)
	}
	rw.WriteContentType(core.PLAIN_TEXT_CTYPE)

	n, err := rw.rw.Write(bytes.Bytes)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteBinary(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.APP_OCTET_STREAM_CTYPE) {
		return 0, fmt.Errorf("cannot write binary: %w", ErrNotAcceptedContentType)
	}
	rw.WriteContentType(core.APP_OCTET_STREAM_CTYPE)

	n, err := rw.rw.Write(bytes.Bytes)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteHTML(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.HTML_CTYPE) {
		return 0, fmt.Errorf("cannot write HTML: %w", ErrNotAcceptedContentType)
	}

	var reader *core.Reader
	var b []byte

	switch val := v.(type) {
	case core.Readable:
		reader = val.Reader()
	default:
		return 0, errors.New("argument not readale")
	}

	d, err := reader.ReadAll()
	if err != nil {
		return -1, err
	}
	b = d.Bytes

	//TODO: check this is valid HTML

	rw.rw.Header().Set("Content-Type", core.HTML_CTYPE)
	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteJS(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.JS_CTYPE) {
		return 0, fmt.Errorf("cannot write JS: %w", ErrNotAcceptedContentType)
	}

	var reader *core.Reader
	var b []byte

	switch val := v.(type) {
	case core.Readable:
		reader = val.Reader()
	default:
		return 0, errors.New("argument not readale")
	}

	d, err := reader.ReadAll()
	if err != nil {
		return -1, err
	}
	b = d.Bytes

	//TODO: check this is valid JS

	rw.rw.Header().Set("Content-Type", core.JS_CTYPE)
	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteCSS(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.CSS_CTYPE) {
		return 0, fmt.Errorf("cannot write CSS: %w", ErrNotAcceptedContentType)
	}

	var reader *core.Reader
	var b []byte

	switch val := v.(type) {
	case core.Readable:
		reader = val.Reader()
	default:
		return 0, errors.New("argument not readale")
	}

	d, err := reader.ReadAll()
	if err != nil {
		return -1, err
	}
	b = d.Bytes

	//TODO: check this is valid CSS

	rw.rw.Header().Set("Content-Type", core.CSS_CTYPE)
	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteJSON(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.JSON_CTYPE) {
		return 0, fmt.Errorf("cannot write JSON: %w", ErrNotAcceptedContentType)
	}

	var reader *core.Reader
	var b []byte

	switch val := v.(type) {
	case core.Readable:
		reader = val.Reader()
	default:
		b = []byte(core.ToJSONWithConfig(ctx, val, &core.ReprConfig{}))
	}

	if len(b) == 0 {
		d, err := reader.ReadAll()
		if err != nil {
			return -1, err
		}
		b = d.Bytes
	}

	if !json.Valid(b) {
		return 0, fmt.Errorf("not valid JSON : %s", string(b))
	}
	rw.rw.Header().Set("Content-Type", core.JSON_CTYPE)
	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteIXON(ctx *core.Context, v core.Value) error {
	rw.assertIsNotFinished()

	if !rw.acceptHeader.Match(core.IXON_CTYPE) {
		return fmt.Errorf("cannot write IXON: %w", ErrNotAcceptedContentType)
	}

	if !v.HasRepresentation(map[uintptr]int{}, &core.ReprConfig{}) {
		return core.ErrNoRepresentation
	}

	rw.rw.Header().Set("Content-Type", core.IXON_CTYPE)

	err := v.WriteRepresentation(ctx, rw.rw, map[uintptr]int{}, &core.ReprConfig{})
	return err
}

func (rw *HttpResponseWriter) BodyWriter() io.Writer {
	rw.assertIsNotFinished()
	return io.Writer(rw.rw)
}

func (rw *HttpResponseWriter) RespWriter() http.ResponseWriter {
	rw.assertIsNotFinished()
	return rw.rw
}

func (rw *HttpResponseWriter) SetCookie(ctx *core.Context, obj *core.Object) error {
	rw.assertIsNotFinished()

	cookie := &http.Cookie{}

	cookie, err := createCookieFromObject(obj)
	if err != nil {
		return err
	}

	http.SetCookie(rw.rw, cookie)
	return nil
}

func (rw *HttpResponseWriter) writeStatus(status core.Int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSet()

	if rw.status >= 0 {
		panic(errors.New("status already written"))
	}
	rw.status = int(status)
	rw.rw.WriteHeader(int(status))
}

func (rw *HttpResponseWriter) WriteStatus(ctx *core.Context, status core.Int) {
	rw.writeStatus(status)
}

func (rw *HttpResponseWriter) WriteError(ctx *core.Context, err core.Error, code core.Int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSet()

	rw.status = int(code)
	defer rw.logger.Print(err)
	http.Error(rw.rw, err.Text(), int(code))
}

func (rw *HttpResponseWriter) AddHeader(ctx *core.Context, k, v core.Str) {
	rw.assertIsNotFinished()

	rw.rw.Header().Add(string(k), string(v))
}

func (rw *HttpResponseWriter) WriteContentType(s string) {
	rw.assertIsNotFinished()
	rw.rw.Header().Set("Content-Type", s)
}

func (rw HttpResponseWriter) Status() int {
	if rw.status < 0 {
		return 200
	}
	return rw.status
}

func (rw *HttpResponseWriter) Finish(ctx *core.Context) {
	rw.assertIsNotFinished()
	rw.finished = true
}

func (rw *HttpResponseWriter) assertStatusNotSet() {
	if rw.status >= 0 {
		panic(ErrStatusAlreadySet)
	}
}

func (rw *HttpResponseWriter) assertIsNotFinished() {
	if rw.finished {
		panic(ErrCannotMutateFinishedResponse)
	}
}

func (rw *HttpResponseWriter) FinalLog() {
	req := rw.request

	duration := time.Since(req.CreationTime)

	rw.logger.Info().
		Str("path", string(req.Path)).
		Dur("duration", duration).
		Int("status", rw.Status()).
		Send()
}
