package core

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestURLPattern(t *testing.T) {
	t.Run("Test", func(t *testing.T) {
		//prefix URL patterns
		{
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab/..."), URL("https://localhost:443/ab"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab/..."), URL("https://localhost:443/ab/c"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab/..."), URL("https://localhost:443/ab/c?q=a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab/..."), URL("https://localhost:443/ab/c#f"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab/..."), URL("https://localhost:443/ab/c?q=a#f"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/..."), URL("wss://localhost:443/"))
			assertPatternTests(t, nil, URLPattern("wss://localhost:443/..."), URL("wss://localhost:443/"))
		}

		//regular URL patterns
		{
			//base cases.
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:443/ab"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:443/ab#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:443/ab#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:443/ab?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:442/ab")) //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab"), URL("https://localhost:443/ab?x=1"))

			//'*' wildcard as first segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/"))   //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/a/")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/a"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a?x=1"))

			//'*' wildcard in first segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/a*"), URL("https://localhost:443/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/aa?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/"))   //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/a/")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:442/a"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443/a?x=1"))

			//escaped '*' wildcard as first segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:443/*"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:443/*#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:443/*#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:443/*?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:442/*a")) //additional character
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:442/"))   //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:442/*/")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:442/*"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/\\*"), URL("https://localhost:443/*?x=1"))

			//'**' wildcard as first segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/aa"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/aa#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/aa#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/aa?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a/a?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:442/"))   //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:442/a/")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:442/a"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a?x=1"))

			//'*' wildcard as second segment (not last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/aa/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/aa/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/aa/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/aa/a?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*"), URL("https://localhost:443//a"))     //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a/")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:442/a/a"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/*/a"), URL("https://localhost:443/a/a?x=1"))

			//'**/a*' wildcard at the end

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**"), URL("https://localhost:443/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/aa"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/aa#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/aa#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/aa?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a/a?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a/"))  //missing leading 'a'
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:442/b"))   //emty segment
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:442/a/b")) //additional slash
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:442/a"))   //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/**/a*"), URL("https://localhost:443/a?x=1"))

			//pattern as first segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/0"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/0#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/0#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/0?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/10"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/10#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/10#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/10?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/a"))  //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/aa")) //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:442/0"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int"), URL("https://localhost:443/0?x=1"))

			//pattern as first segment (not last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/0/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/0/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/0/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/0/a?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/10/a"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/10/a#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/10/a#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/10/a?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/a/a"))  //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/aa/a")) //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:442/0/a"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/0/a?x=1"))
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/%int/a"), URL("https://localhost:443/a/b"))

			//pattern as second segment (last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/0"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/0#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/0#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/0?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/10"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/10#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/10#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/10?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/a"))  //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/aa")) //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:442/a/0"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int"), URL("https://localhost:443/a/0?x=1"))

			//pattern as second segment (not last)

			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/0/b"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/0/b#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/0/b#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/0/b?"))

			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/10/b"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/10/b#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/10/b#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/10/b?"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/a/b"))  //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/aa/b")) //not an integer
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:442/a/0/b"))  //different port
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/0/b?x=1"))
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/a/%int/b"), URL("https://localhost:443/a/10/c"))

			//empty fragment

			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab#"), URL("https://localhost:443/ab"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab#"), URL("https://localhost:443/ab#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab#"), URL("https://localhost:443/ab?"))

			//non-empty fragment

			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab#fragment"), URL("https://localhost:443/ab#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab#fragment"), URL("https://localhost:443/ab?#fragment"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab#fragment"), URL("https://localhost:442/ab"))
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab#fragment"), URL("https://localhost:443/ab?x=1#fragment"))

			//empty query

			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?"), URL("https://localhost:443/ab"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?"), URL("https://localhost:443/ab#"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?"), URL("https://localhost:443/ab#fragment"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?"), URL("https://localhost:443/ab?"))

			//non-empty query

			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?x=1"), URL("https://localhost:443/ab?x=1"))
			assertPatternTests(t, nil, URLPattern("https://localhost:443/ab?x=1"), URL("https://localhost:443/ab?x=1#fragment"))

			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab?x=1"), URL("https://localhost:442/ab"))
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab?x=1"), URL("https://localhost:443/ab?x=1&x=2")) //duplicate param
			assertPatternDoesntTest(t, nil, URLPattern("https://localhost:443/ab?x=1"), URL("https://localhost:443/ab?x=1&y=2"))
		}
	})

	t.Run("Includes", func(t *testing.T) {
		//URL
		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c?q=a")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c#f")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URL("https://localhost:443/ab/c?q=a#f")))

		assert.False(t, URLPattern("https://localhost:443/...").Includes(nil, URL("wss://localhost:443/")))
		assert.True(t, URLPattern("wss://localhost:443/...").Includes(nil, URL("wss://localhost:443/")))

		//URL pattern
		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c?q=a")))

		assert.False(t, URLPattern("https://localhost:443/...").Includes(nil, URLPattern("wss://localhost:443/")))
		assert.True(t, URLPattern("wss://localhost:443/...").Includes(nil, URLPattern("wss://localhost:443/")))

		assert.False(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/...")))
		assert.True(t, URLPattern("https://localhost:443/ab/...").Includes(nil, URLPattern("https://localhost:443/ab/c/...")))
	})

	t.Run("Scheme", func(t *testing.T) {
		assert.EqualValues(t, "https", URLPattern("https://localhost/").Scheme())
		assert.EqualValues(t, "https", URLPattern("https://localhost:443/").Scheme())
		assert.EqualValues(t, "https", URLPattern("https://localhost/...").Scheme())
		assert.EqualValues(t, "https", URLPattern("https://localhost/%ulid").Scheme())
	})

	t.Run("Host", func(t *testing.T) {
		assert.EqualValues(t, "https://localhost", URLPattern("https://localhost/").Host())
		assert.EqualValues(t, "https://localhost", URLPattern("https://localhost/...").Host())
		assert.EqualValues(t, "https://localhost", URLPattern("https://localhost#fragment").Host())
		assert.EqualValues(t, "https://localhost", URLPattern("https://localhost?q=a").Host())
		assert.EqualValues(t, "https://localhost:443", URLPattern("https://localhost:443/").Host())
		assert.EqualValues(t, "https://localhost", URLPattern("https://localhost/%ulid").Host())
	})
}

func TestPathPattern(t *testing.T) {
	t.Run("Test", func(t *testing.T) {

		assertPatternTests(t, nil, PathPattern("/..."), Path("/"))
		assertPatternTests(t, nil, PathPattern("/..."), Path("/f"))
		assertPatternTests(t, nil, PathPattern("/..."), Path("/file.txt"))
		assertPatternTests(t, nil, PathPattern("/..."), Path("/dir/"))
		assertPatternTests(t, nil, PathPattern("/..."), Path("/dir/file.txt"))

		assertPatternTests(t, nil, PathPattern("/dir/..."), Path("/dir/"))
		assertPatternTests(t, nil, PathPattern("/dir/..."), Path("/dir/file.txt"))
		assertPatternDoesntTest(t, nil, PathPattern("/dir/..."), Path("/"))
		assertPatternDoesntTest(t, nil, PathPattern("/dir/..."), Path("/f"))
		assertPatternDoesntTest(t, nil, PathPattern("/dir/..."), Path("/file.txt"))

		assertPatternTests(t, nil, PathPattern("/*"), Path("/"))
		assertPatternTests(t, nil, PathPattern("/*"), Path("/f"))
		assertPatternTests(t, nil, PathPattern("/*"), Path("/file.txt"))
		assertPatternDoesntTest(t, nil, PathPattern("/*"), Path("/dir/"))
		assertPatternDoesntTest(t, nil, PathPattern("/*"), Path("/dir/file.txt"))

		assertPatternTests(t, nil, PathPattern("/[a-z]"), Path("/a"))
		assertPatternDoesntTest(t, nil, PathPattern("/[a-z]"), Path("/aa"))
		assertPatternDoesntTest(t, nil, PathPattern("/[a-z]"), Path("/a0"))
		assertPatternDoesntTest(t, nil, PathPattern("/[a-z]"), Path("/0"))
		assertPatternDoesntTest(t, nil, PathPattern("/[a-z]"), Path("/a/"))
		assertPatternDoesntTest(t, nil, PathPattern("/[a-z]"), Path("/a/a"))

		assertPatternTests(t, nil, PathPattern("/**"), Path("/"))
		assertPatternTests(t, nil, PathPattern("/**"), Path("/f"))
		assertPatternTests(t, nil, PathPattern("/**"), Path("/file.txt"))
		assertPatternTests(t, nil, PathPattern("/**"), Path("/dir/"))
		assertPatternTests(t, nil, PathPattern("/**"), Path("/dir/file.txt"))

		assertPatternTests(t, nil, PathPattern("/**/file.txt"), Path("/file.txt"))
		assertPatternTests(t, nil, PathPattern("/**/file.txt"), Path("/dir/file.txt"))
		assertPatternTests(t, nil, PathPattern("/**/file.txt"), Path("/dir/subdir/file.txt"))
	})
}

func TestHostPatternTest(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		assert.True(t, HostPattern("https://*.com").Test(nil, Host("https://a.com")))
		assert.False(t, HostPattern("https://*.com").Test(nil, Host("://a.com")))
		assert.False(t, HostPattern("https://*.com").Test(nil, Host("ws://a.com")))
		assert.True(t, HostPattern("https://a*.com").Test(nil, Host("https://a.com")))
		assert.True(t, HostPattern("https://a*.com").Test(nil, Host("https://ab.com")))
		assert.False(t, HostPattern("https://*.com").Test(nil, Host("https://sub.a.com")))
		assert.True(t, HostPattern("https://**.com").Test(nil, Host("https://sub.a.com")))
		assert.True(t, HostPattern("https://a.*").Test(nil, Host("https://a.com")))
		assert.False(t, HostPattern("https://sub.*").Test(nil, Host("https://sub.a.com")))
		assert.True(t, HostPattern("https://sub.**").Test(nil, Host("https://sub.a.com")))
		assert.False(t, HostPattern("://*.com").Test(nil, Host("https://a.com")))
		assert.False(t, HostPattern("://*.com:8080").Test(nil, Host("https://a.com")))
		assert.True(t, HostPattern("://*.com").Test(nil, Host("://a.com")))
		assert.True(t, HostPattern("ws://*.com").Test(nil, Host("ws://a.com")))
	})

	p := HostPattern("https://*.com")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "*.com", p.WithoutScheme())

	p = HostPattern("https://*.com:8080")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "*.com:8080", p.WithoutScheme())

	p = HostPattern("https://a*.com")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "a*.com", p.WithoutScheme())

	p = HostPattern("https://**.com")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "**.com", p.WithoutScheme())

	p = HostPattern("https://a*")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "a*", p.WithoutScheme())

	p = HostPattern("https://sub.**")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "sub.**", p.WithoutScheme())

	p = HostPattern("https://sub.*")
	assert.Equal(t, Scheme("https"), p.Scheme())
	assert.True(t, p.HasScheme())
	assert.Equal(t, "sub.*", p.WithoutScheme())

	p = HostPattern("://*.com")
	assert.Equal(t, NO_SCHEME_SCHEME_NAME, p.Scheme())
	assert.False(t, p.HasScheme())
	assert.Equal(t, "*.com", p.WithoutScheme())

	p = HostPattern("://*.com:8080")
	assert.Equal(t, NO_SCHEME_SCHEME_NAME, p.Scheme())
	assert.False(t, p.HasScheme())
	assert.Equal(t, "*.com:8080", p.WithoutScheme())

}

func TestNamedSegmentPathPatternTest(t *testing.T) {
	res := parseEval(t, `%/home/{:username}`)
	patt := res.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		ok   bool
		path Path
	}{
		{false, "/home"},
		{false, "/home/"},
		{true, "/home/user"},
		{false, "/home/user/"},
		{false, "/home/user/e"},
	} {
		t.Run(string(testCase.path), func(t *testing.T) {
			assert.Equal(t, testCase.ok, patt.Test(nil, testCase.path))
		})
	}
}

func TestNamedSegmentPathPatternMatchGroups(t *testing.T) {
	res1 := parseEval(t, `%/home/{:username}`)
	patt1 := res1.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		groups map[string]Serializable
		path   Path
	}{
		{nil, "/home"},
		{nil, "/home/"},
		{map[string]Serializable{"0": Path("/home/user"), "username": Str("user")}, "/home/user"},
		{nil, "/home/user/"},
		{nil, "/home/user/e"},
	} {
		t.Run(string(testCase.path), func(t *testing.T) {
			groups, ok, err := patt1.MatchGroups(nil, testCase.path)
			assert.NoError(t, err)

			if ok != (testCase.groups != nil) {
				assert.FailNow(t, "invalid match")
			}
			assert.Equal(t, testCase.groups, groups)
		})

	}

	res2 := parseEval(t, `%/home/{:username}/`)
	patt2 := res2.(*NamedSegmentPathPattern)

	for _, testCase := range []struct {
		groups map[string]Serializable
		path   Path
	}{
		{nil, "/home"},
		{nil, "/home/"},
		{nil, "/home/user"},
		{map[string]Serializable{"0": Path("/home/user/"), "username": Str("user")}, "/home/user/"},
		{nil, "/home/user/e"},
	} {
		t.Run("pattern ends with slash, "+string(testCase.path), func(t *testing.T) {
			groups, ok, err := patt2.MatchGroups(nil, testCase.path)
			assert.NoError(t, err)

			if ok != (testCase.groups != nil) {
				assert.FailNow(t, "invalid match")
			}
			assert.Equal(t, testCase.groups, groups)
		})

	}
}

func TestRepeatedPatternElementRandom(t *testing.T) {
	t.Run("2 ocurrences of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ExactOcurrence,
			exactCount:        2,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			assert.Equal(t, Str("aa"), patt.Random(ctx).(Str))
		}
	})

	t.Run("optional ocurrence of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.OptionalOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			assert.Equal(t, Str(strings.Repeat("a", len(s))), s)
		}
	})

	t.Run("zero or more ocurrences of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ZeroOrMoreOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			length := len(s)

			assert.Equal(t, Str(strings.Repeat("a", length)), s)
		}
	})

	t.Run("at least one ocurrence of constant string", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})

		patt := RepeatedPatternElement{
			regexp:            nil,
			ocurrenceModifier: parse.ZeroOrMoreOcurrence,
			element:           NewExactStringPattern(Str("a")),
		}

		for i := 0; i < 5; i++ {
			s := patt.Random(ctx).(Str)
			length := len(s)

			assert.Equal(t, Str(strings.Repeat("a", length)), s)
		}
	})
}

func TestSequenceStringPatternRandom(t *testing.T) {
	ctx := NewContext(ContextConfig{})

	patt1 := SequenceStringPattern{
		regexp: nil,
		node:   nil,
		elements: []StringPattern{
			NewExactStringPattern(Str("a")),
			NewExactStringPattern(Str("b")),
		},
	}

	assert.Equal(t, Str("ab"), patt1.Random(ctx))
}

func TestUnionStringPatternRandom(t *testing.T) {
	ctx := NewContext(ContextConfig{})

	patt1 := UnionStringPattern{
		regexp: nil,
		node:   nil,
		cases: []StringPattern{
			NewExactStringPattern(Str("a")),
			NewExactStringPattern(Str("b")),
		},
	}

	for i := 0; i < 5; i++ {
		s := patt1.Random(ctx).(Str)
		assert.True(t, s == "a" || s == "b")
	}

}

// parseEval is a utility function that parses the input string and then evalutes the parsed module.
func parseEval(t *testing.T, s string) Value {

	chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
		NameString: "test",
		CodeString: s,
	}))

	mod, err := parse.ParseChunk(s, "")
	assert.NoError(t, err)
	_, err = StaticCheck(StaticCheckInput{
		State: NewGlobalState(NewContext(ContextConfig{})),
		Node:  mod,
		Chunk: chunk,
	})
	assert.NoError(t, err)

	res, err := TreeWalkEval(mod, NewTreeWalkState(NewDefaultTestContext()))
	assert.NoError(t, err)
	return res
}
