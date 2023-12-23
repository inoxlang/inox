package symbolic

var (
	_ = []UrlHolder{(*Object)(nil)}
)

type UrlHolder interface {
	Serializable
	WithURL(url *URL) UrlHolder
	URL() (*URL, bool)
}

func (o *Object) WithURL(url *URL) UrlHolder {
	copy := *o
	copy.url = url
	return &copy
}

func (o *Object) URL() (*URL, bool) {
	if o.url != nil {
		return o.url, true
	}
	return nil, false
}
