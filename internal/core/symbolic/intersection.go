package symbolic

import (
	"errors"
	"sort"
)

const (
	MAX_INTERSECTION_COMPUTATION_DEPTH = 10
)

var (
	ErrMaxIntersectionComputationDepthExceeded = errors.New("mamximum intersection computation depth exceeded")
	_                                          = []ISpecificIntersection{
		(*Object)(nil),
	}
)

type ISpecificIntersection interface {
	SpecificIntersection(other Value, depth int) (Value, error)
}

func getIntersection(depth int, values ...Value) (Value, error) {
	if depth > MAX_INTERSECTION_COMPUTATION_DEPTH {
		return nil, ErrMaxIntersectionComputationDepthExceeded
	}

	switch len(values) {
	case 0:
		panic("at least 1 value required")
	case 1:
		return values[0], nil
	}

	//move multivalues & intersections at the start
	sort.Slice(values, func(i, j int) bool {
		switch values[i].(type) {
		case IMultivalue:
			return true
		}
		return false
	})

	currentIntersection := values[0]

	for _, value := range values[1:] {
		if itf, ok := currentIntersection.(ISpecificIntersection); ok {
			nextIntersection, err := itf.SpecificIntersection(value, depth+1)
			if err != nil {
				return nil, err
			}
			if nextIntersection != NEVER {
				currentIntersection = nextIntersection
				continue
			}

			//try calling .Intersection() the other way around

			itf2, ok := value.(ISpecificIntersection)
			if !ok {
				return NEVER, nil
			}

			nextIntersection, err = itf2.SpecificIntersection(currentIntersection, depth+1)
			if err != nil {
				return nil, err
			}
			if nextIntersection == NEVER {
				return NEVER, nil
			}
			currentIntersection = nextIntersection
		} else if currentIntersection.Test(value, RecTestCallState{}) {
			currentIntersection = value
		} else if value.Test(currentIntersection, RecTestCallState{}) {
			//current intersection is more narrow
		} else {
			return NEVER, nil
		}
	}

	return currentIntersection, nil
}
