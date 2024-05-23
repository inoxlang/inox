package pathutils

func StripTrailingSlash[S ~string](s S) S {
	if s != "/" && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}

func AppendTrailingSlashIfNotPresent[S ~string](s S) S {
	if s[len(s)-1] != '/' {
		return s + "/"
	}
	return s
}
