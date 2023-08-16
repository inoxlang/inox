package symbolic

var (
	_ = []UrlHolder{(*Object)(nil)}
)

type UrlHolder interface {
	Serializable
	_url()
}

type UrlHolderMixin struct {
}

func (m UrlHolderMixin) _url() {

}
