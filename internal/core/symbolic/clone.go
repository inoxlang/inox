package symbolic

var (
	_ = []ClonableSerializable{
		(*List)(nil), (*RuneSlice)(nil), (*ByteSlice)(nil), (*Option)(nil), (*Dictionary)(nil),
	}
)

type ClonableSerializable interface {
	Serializable
	_clonableSerializable()
}

type ClonableSerializableMixin struct {
}

func (m ClonableSerializableMixin) _clonableSerializable() {

}
