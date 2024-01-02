package http_ns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aohorodnyk/mimeheader"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mimeconsts"

	"github.com/rs/zerolog"

	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

const (
	REQUEST_ID_LOG_FIELD_NAME = "reqID"
)

var (
	ErrNotAcceptedContentType                   = errors.New("not accepted content type")
	ErrCannotMutateWriterOfFinishedResponse     = errors.New("cannot mutate writer of finished response")
	ErrCannotMutateWriterWithDetachedRespWriter = errors.New("cannot mutate writer with detached response writer")
	ErrStatusAlreadySent                        = errors.New("status already sent")
)

type HttpResponseWriter struct {
	request      *HttpRequest
	acceptHeader mimeheader.AcceptHeader
	rw           http.ResponseWriter

	plannedStatus int
	sentStatus    int
	isStatusSent  bool

	finished bool
	detached bool
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
		acceptHeader:  req.ParsedAcceptHeader,
		rw:            rw,
		request:       req,
		plannedStatus: -1,
		logger:        requestLogger,
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
	case "set_cookie":
		return core.WrapGoMethod(rw.SetCookie), true
	case "set_status":
		return core.WrapGoMethod(rw.SetStatus), true
	case "write_headers":
		return core.WrapGoMethod(rw.WriteHeaders), true
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
	return http_ns_symb.HTTP_RESP_WRITER_PROPNAMES
}

func (rw *HttpResponseWriter) headers() http.Header {
	return rw.rw.Header()
}

func (rw *HttpResponseWriter) WritePlainText(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.PLAIN_TEXT_CTYPE) {
		return 0, fmt.Errorf("cannot write plain text: %w", ErrNotAcceptedContentType)
	}
	rw.SetContentType(mimeconsts.PLAIN_TEXT_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(bytes.UnderlyingBytes())
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteBinary(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.APP_OCTET_STREAM_CTYPE) {
		return 0, fmt.Errorf("cannot write binary: %w", ErrNotAcceptedContentType)
	}
	rw.SetContentType(mimeconsts.APP_OCTET_STREAM_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(bytes.UnderlyingBytes())
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteHTML(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.HTML_CTYPE) {
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
	b = d.UnderlyingBytes()

	//TODO: check this is valid HTML

	rw.rw.Header().Set("Content-Type", mimeconsts.HTML_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteJS(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.JS_CTYPE) {
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
	b = d.UnderlyingBytes()

	//TODO: check this is valid JS

	rw.rw.Header().Set("Content-Type", mimeconsts.JS_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteCSS(ctx *core.Context, v core.Value) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.CSS_CTYPE) {
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
	b = d.UnderlyingBytes()

	//TODO: check this is valid CSS

	rw.rw.Header().Set("Content-Type", mimeconsts.CSS_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteJSON(ctx *core.Context, v core.Serializable) (core.Int, error) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.JSON_CTYPE) {
		return 0, fmt.Errorf("cannot write JSON: %w", ErrNotAcceptedContentType)
	}

	var reader *core.Reader
	var b []byte

	switch val := v.(type) {
	case core.Readable:
		reader = val.Reader()
	default:
		b = []byte(core.ToJSONWithConfig(ctx, val, core.JSONSerializationConfig{
			ReprConfig: &core.ReprConfig{},
		}))
	}

	if len(b) == 0 {
		d, err := reader.ReadAll()
		if err != nil {
			return -1, err
		}
		b = d.UnderlyingBytes()
	}

	if !json.Valid(b) {
		return 0, fmt.Errorf("not valid JSON : %s", string(b))
	}

	rw.rw.Header().Set("Content-Type", mimeconsts.JSON_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	n, err := rw.rw.Write(b)
	return core.Int(n), err
}

func (rw *HttpResponseWriter) WriteIXON(ctx *core.Context, v core.Serializable) error {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	if !rw.acceptHeader.Match(mimeconsts.IXON_CTYPE) {
		return fmt.Errorf("cannot write IXON: %w", ErrNotAcceptedContentType)
	}

	rw.rw.Header().Set("Content-Type", mimeconsts.IXON_CTYPE)
	rw.writeHeadersWithPlannedStatus()

	err := v.WriteRepresentation(ctx, rw.rw, &core.ReprConfig{}, 0)
	return err
}

// DetachBodyWriter writes the headers and the planned status if they have not been sent yet,
// then it detachs the underlying response writer and returns it. The HttpResponseWriter should
// not be used afterwards.
func (rw *HttpResponseWriter) DetachBodyWriter() io.Writer {
	rw.assertIsNotFinished()

	if !rw.IsStatusSent() {
		rw.writeHeadersWithPlannedStatus()
	}

	rw.detached = true
	w := io.Writer(rw.rw)
	rw.rw = nil
	return w
}

// DetachBodyWriter detachs the underlying response writer and returns it.
// The HttpResponseWriter should not be used afterwards.
func (rw *HttpResponseWriter) DetachRespWriter() http.ResponseWriter {
	rw.assertIsNotFinished()
	rw.detached = true
	w := rw.rw
	rw.rw = nil
	return w
}

func (rw *HttpResponseWriter) SetWriteDeadline(timeout time.Duration) {
	http.NewResponseController(rw.rw).SetWriteDeadline(time.Now().Add(timeout))
}

func (rw *HttpResponseWriter) SetCookie(ctx *core.Context, obj *core.Object) error {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	cookie := &http.Cookie{}

	cookie, err := createCookieFromObject(ctx, obj)
	if err != nil {
		return err
	}

	http.SetCookie(rw.rw, cookie)
	return nil
}

func (rw *HttpResponseWriter) SetStatus(ctx *core.Context, status core.Int) {
	rw.setStatus(ctx, int(status))
}

func (rw *HttpResponseWriter) setStatus(ctx *core.Context, status int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.plannedStatus = int(status)
}

func (rw *HttpResponseWriter) WriteHeaders(ctx *core.Context, status *core.OptionalParam[core.Int]) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	statusCode := rw.PlannedStatus()

	if status != nil {
		statusCode = int(status.Value)
	}

	rw.writeHeaders(int(statusCode))
}

func (rw *HttpResponseWriter) writeHeaders(status int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.sentStatus = status
	rw.isStatusSent = true
	rw.rw.WriteHeader(status)
}

func (rw *HttpResponseWriter) writeHeadersWithPlannedStatus() {
	rw.writeHeaders(rw.PlannedStatus())
}

func (rw *HttpResponseWriter) WriteError(ctx *core.Context, err core.Error, code core.Int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.plannedStatus = int(code)
	defer rw.logger.Print(err)

	headers := rw.headers()

	headers.Set("Content-Type", "text/plain; charset=utf-8")
	headers.Set("X-Content-Type-Options", "nosniff")
	rw.writeHeadersWithPlannedStatus()
}

func (rw *HttpResponseWriter) AddHeader(ctx *core.Context, k, v core.Str) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.rw.Header().Add(string(k), string(v))
}

func (rw *HttpResponseWriter) SetContentType(s string) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.rw.Header().Set("Content-Type", s)
}

func (rw HttpResponseWriter) PlannedStatus() int {
	if rw.plannedStatus <= 0 {
		return 200
	}
	return rw.plannedStatus
}

func (rw HttpResponseWriter) SentStatus() int {
	if rw.sentStatus == 0 {
		return 200
	}
	return rw.sentStatus
}

func (rw *HttpResponseWriter) Finish(ctx *core.Context) {
	rw.assertIsNotFinished()
	rw.finished = true
}

func (rw *HttpResponseWriter) IsStatusSent() bool {
	return rw.isStatusSent
}

func (rw *HttpResponseWriter) assertStatusNotSent() {
	if rw.isStatusSent {
		panic(ErrStatusAlreadySent)
	}
}

func (rw *HttpResponseWriter) isPlannedStatusSet() bool {
	return rw.plannedStatus > 0
}

func (rw *HttpResponseWriter) assertIsNotFinished() {
	if rw.finished {
		panic(ErrCannotMutateWriterOfFinishedResponse)
	}
	if rw.detached {
		panic(ErrCannotMutateWriterWithDetachedRespWriter)
	}
}

func (rw *HttpResponseWriter) FinalLog() {
	req := rw.request

	duration := time.Since(req.CreationTime)

	rw.logger.Info().
		Str("path", string(req.Path)).
		Dur("duration", duration).
		Int("status", rw.SentStatus()).
		Send()
}
