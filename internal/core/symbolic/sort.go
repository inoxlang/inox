package symbolic

type Order int

const (
	AscendingOrder Order = iota + 1
	DescendingOrder
	LexicographicOrder
	ReverseLexicographicOrder
)

func OrderFromString(name string) (Order, bool) {
	switch name {
	case "asc":
		return AscendingOrder, true
	case "desc":
		return DescendingOrder, true
	case "lex":
		return LexicographicOrder, true
	case "revlex":
		return ReverseLexicographicOrder, true
	}

	return 0, false
}
