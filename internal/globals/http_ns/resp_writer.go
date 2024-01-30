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

type ResponseWriter struct {
	request      *Request
	acceptHeader mimeheader.AcceptHeader
	rw           http.ResponseWriter

	plannedStatus int
	sentStatus    int
	isStatusSent  bool

	finished bool
	detached bool
	logger   zerolog.Logger
}

func NewResponseWriter(req *Request, rw http.ResponseWriter, serverLogger zerolog.Logger) *ResponseWriter {
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

	return &ResponseWriter{
		acceptHeader:  req.ParsedAcceptHeader,
		rw:            rw,
		request:       req,
		plannedStatus: -1,
		logger:        requestLogger,
	}
}

func (rw *ResponseWriter) GetGoMethod(name string) (*core.GoFunction, bool) {
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

func (rw *ResponseWriter) Prop(ctx *core.Context, name string) core.Value {
	rw.assertIsNotFinished()

	method, ok := rw.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, rw))
	}
	return method
}

func (*ResponseWriter) SetProp(ctx *core.Context, name string, value core.Value) error {

	return core.ErrCannotSetProp
}

func (rw *ResponseWriter) PropertyNames(ctx *core.Context) []string {
	rw.assertIsNotFinished()
	return http_ns_symb.HTTP_RESP_WRITER_PROPNAMES
}

func (rw *ResponseWriter) headers() http.Header {
	return rw.rw.Header()
}

func (rw *ResponseWriter) WritePlainText(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
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

func (rw *ResponseWriter) WriteBinary(ctx *core.Context, bytes *core.ByteSlice) (core.Int, error) {
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

func (rw *ResponseWriter) WriteHTML(ctx *core.Context, v core.Value) (core.Int, error) {
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

func (rw *ResponseWriter) WriteJS(ctx *core.Context, v core.Value) (core.Int, error) {
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

func (rw *ResponseWriter) WriteCSS(ctx *core.Context, v core.Value) (core.Int, error) {
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

func (rw *ResponseWriter) WriteJSON(ctx *core.Context, v core.Serializable) (core.Int, error) {
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

// DetachBodyWriter writes the headers and the planned status if they have not been sent yet,
// then it detachs the underlying response writer and returns it. The HttpResponseWriter should
// not be used afterwards.
func (rw *ResponseWriter) DetachBodyWriter() io.Writer {
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
func (rw *ResponseWriter) DetachRespWriter() http.ResponseWriter {
	rw.assertIsNotFinished()
	rw.detached = true
	w := rw.rw
	rw.rw = nil
	return w
}

func (rw *ResponseWriter) SetWriteDeadline(timeout time.Duration) {
	http.NewResponseController(rw.rw).SetWriteDeadline(time.Now().Add(timeout))
}

func (rw *ResponseWriter) SetCookie(ctx *core.Context, obj *core.Object) error {
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

func (rw *ResponseWriter) SetStatus(ctx *core.Context, status StatusCode) {
	rw.setStatus(ctx, int(status))
}

func (rw *ResponseWriter) setStatus(ctx *core.Context, status int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.plannedStatus = int(status)
}

func (rw *ResponseWriter) WriteHeaders(ctx *core.Context, status *core.OptionalParam[StatusCode]) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	statusCode := rw.PlannedStatus()

	if status != nil {
		statusCode = int(status.Value)
	}

	rw.writeHeaders(int(statusCode))
}

func (rw *ResponseWriter) writeHeaders(status int) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.sentStatus = status
	rw.isStatusSent = true
	rw.rw.WriteHeader(status)
}

func (rw *ResponseWriter) writeHeadersWithPlannedStatus() {
	rw.writeHeaders(rw.PlannedStatus())
}

func (rw *ResponseWriter) WriteError(ctx *core.Context, err core.Error, code StatusCode) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.plannedStatus = int(code)
	defer rw.logger.Print(err)

	headers := rw.headers()

	headers.Set("Content-Type", "text/plain; charset=utf-8")
	headers.Set("X-Content-Type-Options", "nosniff")
	rw.writeHeadersWithPlannedStatus()
}

func (rw *ResponseWriter) AddHeader(ctx *core.Context, k, v core.String) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.rw.Header().Add(string(k), string(v))
}

func (rw *ResponseWriter) SetContentType(s string) {
	rw.assertIsNotFinished()
	rw.assertStatusNotSent()

	rw.rw.Header().Set("Content-Type", s)
}

func (rw ResponseWriter) PlannedStatus() int {
	if rw.plannedStatus <= 0 {
		return 200
	}
	return rw.plannedStatus
}

func (rw ResponseWriter) SentStatus() int {
	if rw.sentStatus == 0 {
		return 200
	}
	return rw.sentStatus
}

func (rw *ResponseWriter) Finish(ctx *core.Context) {
	rw.assertIsNotFinished()
	rw.finished = true
}

func (rw *ResponseWriter) IsStatusSent() bool {
	return rw.isStatusSent
}

func (rw *ResponseWriter) assertStatusNotSent() {
	if rw.isStatusSent {
		panic(ErrStatusAlreadySent)
	}
}

func (rw *ResponseWriter) isPlannedStatusSet() bool {
	return rw.plannedStatus > 0
}

func (rw *ResponseWriter) assertIsNotFinished() {
	if rw.finished {
		panic(ErrCannotMutateWriterOfFinishedResponse)
	}
	if rw.detached {
		panic(ErrCannotMutateWriterWithDetachedRespWriter)
	}
}

func (rw *ResponseWriter) FinalLog() {
	req := rw.request

	duration := time.Since(req.CreationTime)

	rw.logger.Info().
		Str("path", string(req.Path)).
		Dur("duration", duration).
		Int("status", rw.SentStatus()).
		Send()
}
