package internal

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/inox-project/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCookieJar(t *testing.T) {
	jar := utils.Must(newCookieJar())
	parse := func(u string) *url.URL {
		return utils.Must(url.Parse(u))
	}

	//
	jar.SetCookies(parse("https://example.com/"), []*http.Cookie{{Name: "a", Value: "1", Path: "/"}})

	m := jar.AllCookies()
	//check
	assert.Len(t, m, 1)
	assert.Contains(t, m, "https://example.com/")

	//
	jar.SetCookies(parse("https://example.com/e"), []*http.Cookie{{Name: "b", Value: "1", Path: "/e"}})

	m = jar.AllCookies()
	//check
	assert.Len(t, m, 2)
	assert.Contains(t, m, "https://example.com/")
	assert.Contains(t, m, "https://example.com/e")

	//
	jar.SetCookies(parse("https://example.org/e"), []*http.Cookie{{Name: "b", Value: "1", Path: "/e"}})

	m = jar.AllCookies()
	//check
	assert.Len(t, m, 3)
	assert.Contains(t, m, "https://example.com/")
	assert.Contains(t, m, "https://example.com/e")
	assert.Contains(t, m, "https://example.org/e")
}
