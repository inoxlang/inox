package http_ns

import (
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MIN_SESSION_ID_BYTE_COUNT      = 16
	MAX_SESSION_ID_BYTE_COUNT      = 32
	DEFAULT_SESSION_ID_BYTE_COUNT  = MIN_SESSION_ID_BYTE_COUNT
	DEFAULT_SESSION_ID_COOKIE_NAME = "session-id"

	SESSION_CTX_DATA_KEY = core.Path("/session")
)

var (
	MIN_SESSION_ID_LEN   = hex.EncodedLen(MIN_SESSION_ID_BYTE_COUNT)
	MAX_SESSION_ID_LEN   = hex.EncodedLen(MAX_SESSION_ID_BYTE_COUNT)
	ErrSessionNotFound   = errors.New("session not found")
	ErrSessionIdTooLong  = errors.New("session id is too long")
	ErrSessionIdTooShort = errors.New("session id is too short")
)

func (server *HttpsServer) getSession(ctx *core.Context, req *Request) (*core.Object, error) {

	if server.sessions == nil {
		return nil, ErrSessionNotFound
	}

	for _, cookie := range req.Cookies {
		if cookie.Name == DEFAULT_SESSION_ID_COOKIE_NAME {
			if len(cookie.Value) > MAX_SESSION_ID_LEN {
				return nil, ErrSessionIdTooLong
			}
			if len(cookie.Value) < MIN_SESSION_ID_LEN {
				return nil, ErrSessionIdTooShort
			}

			var array [MAX_SESSION_ID_BYTE_COUNT + 2]byte
			key := array[:0]
			key = append(key, '"')
			key = append(key, cookie.Value...)
			key = append(key, '"')

			session, ok := server.sessions.Get(ctx, core.String(utils.BytesAsString(key[:])))
			if ok {
				return session.(*core.Object), nil
			}
			//_ = session
			//_ = ok
			return nil, ErrSessionNotFound
		}
	}

	return nil, ErrSessionNotFound
}

func addSessionIdCookie(rw *ResponseWriter, sessionId string) {
	http.SetCookie(rw.rw, &http.Cookie{
		Name:     DEFAULT_SESSION_ID_COOKIE_NAME,
		Value:    sessionId,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		HttpOnly: true,
	})
}

func isValidHexSessionID(s string) bool {
	if len(s) < MIN_SESSION_ID_BYTE_COUNT || len(s) > MAX_SESSION_ID_BYTE_COUNT || (len(s)%2) != 0 {
		return false
	}

	for i := 0; i < len(s); i++ {
		if !utils.IsHexDigit(s[i]) {
			return false
		}
	}

	return true
}

func randomSessionID() string {
	var sessionId [DEFAULT_SESSION_ID_BYTE_COUNT]byte
	_, err := core.CryptoRandSource.Read(sessionId[:])

	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(sessionId[:])
}
