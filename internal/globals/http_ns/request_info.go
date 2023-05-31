package http_ns

import (
	"net"
	"strings"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/oklog/ulid/v2"
)

// should not contain super sensitive information (full cookie, email address, ...)
type IncomingRequestInfo struct {
	ULID     ulid.ULID
	Path     core.Path
	Hostname string
	Method   string
	//VMethod                  VMethod
	//ResourceTypeName         ModelName
	ContentType        string
	RemoteAddrAndPort  string
	Referer            string
	URL                core.URL
	UserAgent          string
	ResponseStatusCode int
	//TimeZone           string
	//UserLang           string

	CreationTime                    time.Time
	EndTime                         time.Time //rename ?
	HandlingDurationMillis          int64
	SessionCookieStart              string
	CookieNames                     []string
	Errors                          []string
	Info                            []string
	SetCookieHeaderValueStart       string
	HeaderNames                     []string
	SeeOtherRedirectURL             string
	ResourceWaitingDurationMicrosec int64
	SentBodyBytes                   int
}

func (reqInfo IncomingRequestInfo) RemoteIpAddress() string {
	ip, _, _ := net.SplitHostPort(reqInfo.RemoteAddrAndPort)
	return ip
}

func NewIncomingRequestInfo(r *HttpRequest) *IncomingRequestInfo {
	req := r.request

	var cookieNames []string

	cookies := req.Cookies()
	for _, cookie := range cookies {
		cookieNames = append(cookieNames, cookie.Name)
	}

	now := time.Now()

	headerNames := make([]string, 0, len(req.Header))
	for name, _ := range req.Header {
		headerNames = append(headerNames, name)
	}

	hostname, _, err := net.SplitHostPort(req.Host)
	if err != nil {
		if strings.Contains(err.Error(), "missing port") {
			hostname = req.Host
		} else {
			hostname = "failed-to-split-host-port"
		}
	}

	return &IncomingRequestInfo{
		ULID:              ulid.Make(),
		Path:              r.Path,
		Hostname:          hostname,
		Method:            string(r.Method),
		ContentType:       req.Header.Get("Content-Type"),
		RemoteAddrAndPort: req.RemoteAddr,
		Referer:           req.Header.Get("Referer"),
		URL:               r.URL,
		UserAgent:         req.Header.Get("User-Agent"),
		CookieNames:       cookieNames,
		CreationTime:      now,
		//TimeZone:          timezone,
		//UserLang:          lang,
		HeaderNames: headerNames,
	}
}
