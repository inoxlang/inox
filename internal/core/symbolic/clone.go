package symbolic

var (
	_ = []PseudoClonable{
		(*List)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Option)(nil), (*Dictionary)(nil),
	}
)

type PseudoClonable interface {
	Serializable
	_pseudoClonable()
}

type PseudoClonableMixin struct {
}

func (m PseudoClonableMixin) _pseudoClonable() {

}
