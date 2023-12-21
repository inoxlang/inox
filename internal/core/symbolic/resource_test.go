package symbolic

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
)

func TestPath(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyPath := &Path{}
		assertTest(t, anyPath, anyPath)
		assertTest(t, anyPath, &Path{})
		assertTestFalse(t, anyPath, &String{})
		assertTestFalse(t, anyPath, &Int{})

		anyAbsPath := ANY_ABS_PATH
		assertTest(t, anyAbsPath, anyAbsPath)
		assertTest(t, anyAbsPath, NewPath("/"))
		assertTest(t, anyAbsPath, NewPath("/1"))
		assertTestFalse(t, anyAbsPath, NewPath("./1"))
		assertTestFalse(t, anyAbsPath, anyPath)
		assertTestFalse(t, anyAbsPath, &String{})

		anyDirPath := ANY_DIR_PATH
		assertTest(t, anyDirPath, anyDirPath)
		assertTest(t, anyDirPath, NewPath("/"))
		assertTest(t, anyDirPath, NewPath("./"))
		assertTest(t, anyDirPath, NewPath("./dir/"))
		assertTestFalse(t, anyDirPath, NewPath("/1"))
		assertTestFalse(t, anyDirPath, NewPath("./1"))
		assertTestFalse(t, anyDirPath, anyPath)
		assertTestFalse(t, anyDirPath, anyAbsPath)

		pathWithValue := NewPath("/")
		assertTest(t, pathWithValue, pathWithValue)
		assertTest(t, pathWithValue, NewPath("/"))
		assertTestFalse(t, pathWithValue, NewPath("/1"))
		assertTestFalse(t, pathWithValue, NewPath("./"))
		assertTestFalse(t, pathWithValue, NewPathMatchingPattern(NewPathPattern("/...")))
		assertTestFalse(t, pathWithValue, anyDirPath)
		assertTestFalse(t, pathWithValue, anyPath)
		assertTestFalse(t, pathWithValue, anyAbsPath)

		pathMatchingPatternWithValue := NewPathMatchingPattern(NewPathPattern("/..."))
		assertTest(t, pathMatchingPatternWithValue, pathMatchingPatternWithValue)
		assertTest(t, pathMatchingPatternWithValue, NewPath("/"))
		assertTest(t, pathMatchingPatternWithValue, NewPath("/1"))
		assertTest(t, pathMatchingPatternWithValue, NewPath("/1/"))
		assertTestFalse(t, pathMatchingPatternWithValue, NewPath("./"))
		assertTestFalse(t, pathMatchingPatternWithValue, anyDirPath)
		assertTestFalse(t, pathMatchingPatternWithValue, anyPath)
		assertTestFalse(t, pathMatchingPatternWithValue, anyAbsPath)

		pathMatchingPatternWithNode := NewPathMatchingPattern(&PathPattern{node: &parse.PathPatternExpression{}})
		assertTest(t, pathMatchingPatternWithNode, pathMatchingPatternWithNode)
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/"))
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/1"))
		assertTestFalse(t, pathMatchingPatternWithNode, NewPath("/1/"))
		assertTestFalse(t, pathMatchingPatternWithValue, NewPath("./"))
		assertTestFalse(t, pathMatchingPatternWithNode, anyPath)
		assertTestFalse(t, pathMatchingPatternWithNode, anyAbsPath)
		assertTestFalse(t, pathMatchingPatternWithNode, anyDirPath)
	})

}

func TestURL(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyURL := &URL{}
		assertTest(t, anyURL, anyURL)
		assertTest(t, anyURL, &URL{})
		assertTestFalse(t, anyURL, &String{})
		assertTestFalse(t, anyURL, &Int{})

		urlWithValue := NewUrl("https://example.com/")
		assertTest(t, urlWithValue, urlWithValue)
		assertTest(t, urlWithValue, NewUrl("https://example.com/"))
		assertTestFalse(t, urlWithValue, NewUrl("https://example.com/1"))
		assertTestFalse(t, urlWithValue, NewUrl("https://localhost/"))
		assertTestFalse(t, urlWithValue, NewUrlMatchingPattern(NewUrlPattern("https://example.com/")))

		urlMatchingPatternWithValue := NewUrlMatchingPattern(NewUrlPattern("https://example.com/..."))
		assertTest(t, urlMatchingPatternWithValue, urlMatchingPatternWithValue)
		assertTest(t, urlMatchingPatternWithValue, NewUrl("https://example.com/"))
		assertTest(t, urlMatchingPatternWithValue, NewUrl("https://example.com/1"))
		assertTestFalse(t, urlMatchingPatternWithValue, NewUrl("https://localhost/"))
		assertTestFalse(t, urlMatchingPatternWithValue, anyURL)
	})

}

func TestHost(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyHost := &Host{}
		assertTest(t, anyHost, anyHost)
		assertTest(t, anyHost, &Host{})
		assertTestFalse(t, anyHost, &String{})
		assertTestFalse(t, anyHost, &Int{})

		hostWithValue := NewHost("https://example.com")
		assertTest(t, hostWithValue, hostWithValue)
		assertTest(t, hostWithValue, NewHost("https://example.com"))
		assertTestFalse(t, hostWithValue, NewHost("https://localhost"))
		assertTestFalse(t, hostWithValue, NewHostMatchingPattern(NewHostPattern("https://example.com")))
		assertTestFalse(t, hostWithValue, ANY_HTTPS_HOST)
		assertTestFalse(t, hostWithValue, ANY_HTTP_HOST)

		schemelessHostWithValue := NewHost("://example.com")
		assertTest(t, schemelessHostWithValue, schemelessHostWithValue)
		assertTest(t, schemelessHostWithValue, NewHost("://example.com"))
		assertTestFalse(t, schemelessHostWithValue, NewHost("https://example.com"))
		assertTestFalse(t, schemelessHostWithValue, NewHost("https://localhost"))
		assertTestFalse(t, schemelessHostWithValue, NewHostMatchingPattern(NewHostPattern("https://example.com")))
		assertTestFalse(t, schemelessHostWithValue, ANY_HTTPS_HOST)
		assertTestFalse(t, schemelessHostWithValue, ANY_HTTP_HOST)

		hostMatchingPatternWithValue := NewHostMatchingPattern(NewHostPattern("https://example.com"))
		assertTest(t, hostMatchingPatternWithValue, hostMatchingPatternWithValue)
		assertTest(t, hostMatchingPatternWithValue, NewHost("https://example.com"))
		assertTestFalse(t, hostMatchingPatternWithValue, NewHost("https://exemple.com"))
		assertTestFalse(t, hostMatchingPatternWithValue, NewHost("https://localhost/"))
		assertTestFalse(t, hostMatchingPatternWithValue, anyHost)
		assertTestFalse(t, hostMatchingPatternWithValue, ANY_HTTPS_HOST)
		assertTestFalse(t, hostMatchingPatternWithValue, ANY_HTTP_HOST)

		//check ANY_HTTP_HOST
		assertTest(t, ANY_HTTP_HOST, ANY_HTTP_HOST)
		assertTest(t, ANY_HTTP_HOST, NewHost("http://example.com"))
		assertTestFalse(t, ANY_HTTP_HOST, NewHost("https://exemple.com"))
		assertTestFalse(t, ANY_HTTP_HOST, NewHost("https://localhost/"))
		assertTestFalse(t, ANY_HTTP_HOST, anyHost)
		assertTestFalse(t, ANY_HTTP_HOST, ANY_HTTPS_HOST)
		assertTestFalse(t, ANY_HTTP_HOST, NewHostMatchingPattern(NewHostPattern("://example.com")))

		//check ANY_HTTPS_HOST
		assertTest(t, ANY_HTTPS_HOST, ANY_HTTPS_HOST)
		assertTest(t, ANY_HTTPS_HOST, NewHost("https://example.com"))
		assertTestFalse(t, ANY_HTTPS_HOST, NewHost("http://exemple.com"))
		assertTestFalse(t, ANY_HTTPS_HOST, NewHost("http://localhost/"))
		assertTestFalse(t, ANY_HTTPS_HOST, anyHost)
		assertTestFalse(t, ANY_HTTPS_HOST, ANY_HTTP_HOST)
		assertTestFalse(t, ANY_HTTPS_HOST, NewHostMatchingPattern(NewHostPattern("://example.com")))
	})

}
