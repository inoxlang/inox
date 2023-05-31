package http_ns

import (
	"fmt"
	"net/http"
	_cookiejar "net/http/cookiejar"
	"net/url"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/publicsuffix"
)

type cookiejar struct {
	jar  http.CookieJar
	urls map[string]bool
	lock sync.Mutex
}

func newCookieJar() (*cookiejar, error) {
	j, err := _cookiejar.New(&_cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	jar := cookiejar{
		jar:  j,
		urls: make(map[string]bool),
	}

	return &jar, nil
}

func (jar *cookiejar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.lock.Lock()
	defer jar.lock.Unlock()

	jar.jar.SetCookies(u, cookies)
	jar.urls[u.String()] = true
}

func (jar *cookiejar) Cookies(u *url.URL) []*http.Cookie {
	return jar.jar.Cookies(u)
}

func (jar *cookiejar) AllCookies() map[string][]*http.Cookie {

	result := map[string][]*http.Cookie{}
	encounteredCookies := map[*http.Cookie]bool{}

	for urlStr := range jar.urls {
		u := utils.Must(url.Parse(urlStr))
		cookies := jar.jar.Cookies(u)

		for _, cookie := range cookies {

			if encounteredCookies[cookie] {
				continue
			}

			encounteredCookies[cookie] = true
			result[urlStr] = append(result[urlStr], cookie)
		}
	}

	return result
}

func createCookieFromObject(obj *core.Object) (*http.Cookie, error) {
	const ERROR_PREFIX = "create cookie:"

	cookie := &http.Cookie{
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	for k, v := range obj.EntryMap() {
		switch k {
		case "domain":
			host, ok := v.(core.Host)
			if !ok {
				return nil, fmt.Errorf(ERROR_PREFIX+" .domain should be a HTTPHost not a(n) %T", v)
			}
			cookie.Domain = host.WithoutScheme()
		case "name":
			name, ok := v.(core.Str)
			if !ok {
				return nil, fmt.Errorf(ERROR_PREFIX+" .name should be a string not a(n) %T", v)
			}
			cookie.Name = string(name)
		case "value":
			value, ok := v.(core.Str)
			if !ok {
				return nil, fmt.Errorf(ERROR_PREFIX+" .value should be a string not a(n) %T", v)
			}
			cookie.Value = string(value)
		}
	}

	if cookie.Name == "" {
		return nil, fmt.Errorf(ERROR_PREFIX + " missing cookie's .name")
	}

	if cookie.Value == "" {
		return nil, fmt.Errorf(ERROR_PREFIX + " missing cookie's .value")
	}

	return cookie, nil
}

func createObjectFromCookie(ctx *core.Context, cookie http.Cookie) *core.Object {
	obj := &core.Object{}
	if cookie.Domain != "" {
		obj.SetProp(ctx, "domain", core.Host("://"+cookie.Domain))
	} else {
		obj.SetProp(ctx, "domain", core.Nil)
	}

	obj.SetProp(ctx, "name", core.Str(cookie.Name))
	obj.SetProp(ctx, "value", core.Str(cookie.Value))

	return obj
}
