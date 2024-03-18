package scaffolding

import (
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestWriteTemplate(t *testing.T) {

	fls := fs_ns.NewMemFilesystem(1_000_000)

	if !assert.NoError(t, WriteTemplate("web-app-min", fls)) {
		return
	}

	_, err := fls.Stat("/main.ix")
	if !assert.NoError(t, err) {
		return
	}

	content, err := util.ReadFile(fls, "/static/styles/main.css")

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, MAIN_CSS, string(content))
}
