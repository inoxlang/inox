package symbolic

import "github.com/inoxlang/inox/internal/utils"

// widenOrAny returns the widened value of the passed value, if widening is not possible any is returned.
func widenOrAny(value SymbolicValue) SymbolicValue {
	if value.IsWidenable() {
		widened, _ := value.Widen()
		return widened
	}
	return ANY
}

// join values joins a list of values into a single value by searching for equality/inclusion, the passed list is never modified.
func joinValues(values []SymbolicValue) SymbolicValue {

	// if one of the value is any we just return any
	for _, val := range values {
		if IsAny(val) {
			return ANY
		}
	}

	switch len(values) {
	case 0:
		panic("at least 1 value required")
	case 1:
		return values[0]
	default:
		copy_ := make([]SymbolicValue, len(values))
		copy(copy_, values)
		values = copy_

		// we flatten the list by spreading elements of any MultiValue found
	flattening:
		for {
			for i, val := range values {
				if multiVal, ok := val.(*Multivalue); ok {
					updated := make([]SymbolicValue, len(values)+len(multiVal.values)-1)
					copy(updated[:i], values[:i])
					copy(updated[i:i+len(multiVal.values)], multiVal.values)
					copy(updated[i+len(multiVal.values):], values[i+1:])
					values = updated
					continue flattening
				}
			}

			break
		}

		// merge
		for {
			var removed []int

			for i, val1 := range values {
				if utils.SliceContains(removed, i) {
					continue
				}

				for j, val2 := range values {
					if i != j && val1.Test(val2) {
						if !utils.SliceContains(removed, j) {
							removed = append(removed, j)
						}
					}
				}
			}

			if len(removed) == 0 {
				break
			}

			var newValues = make([]SymbolicValue, 0, len(values)-len(removed))

			for i, val := range values {
				if utils.SliceContains(removed, i) {
					continue
				}
				newValues = append(newValues, val)
			}

			values = newValues
		}

		if len(values) == 1 {
			return values[0]
		}
		return NewMultivalue(values...)
	}
}

// narrowOut narrows out narrowedOut of toNarrow
func narrowOut(narrowedOut SymbolicValue, toNarrow SymbolicValue) SymbolicValue {
	switch n := toNarrow.(type) {
	case *Multivalue:
		var remainingValues []SymbolicValue

		for _, val := range n.values {
			if narrowedOut.Test(val) {
				continue
			}
			remainingValues = append(remainingValues, val)
		}

		if len(remainingValues) == 1 {
			return remainingValues[0]
		}

		return NewMultivalue(remainingValues...)
	case IMultivalue:
		return narrowOut(narrowedOut, n.OriginalMultivalue())
	}

	return toNarrow
}
