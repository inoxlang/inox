package symbolic

var (
	_ = []PotentiallyConcretizable{
		(*Bool)(nil), (*Int)(nil), (*Float)(nil), Nil,

		(*ByteCount)(nil), (*LineCount)(nil), (*ByteRate)(nil), (*SimpleRate)(nil), (*Duration)(nil), (*Date)(nil),

		(*Rune)(nil), (*String)(nil), (*Path)(nil), (*URL)(nil), (*Host)(nil), (*Scheme)(nil),
		(*Identifier)(nil),
		(*PropertyName)(nil),

		(*StringConcatenation)(nil),

		(*RuneSlice)(nil), (*ByteSlice)(nil),

		(*Object)(nil), (*Record)(nil), (*List)(nil), (*Tuple)(nil), (*KeyList)(nil), (*Dictionary)(nil),

		(*ObjectPattern)(nil), (*RecordPattern)(nil), (*ListPattern)(nil), (*TuplePattern)(nil),
		(*TypePattern)(nil),

		(*InoxFunction)(nil),

		(*FileInfo)(nil),

		(*Option)(nil),
	}
)

type PotentiallyConcretizable interface {
	IsConcretizable() bool
}
