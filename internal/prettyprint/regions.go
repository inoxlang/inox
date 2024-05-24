package prettyprint

type RegionKind uint8

const (
	ParamNameTypeRegion RegionKind = iota + 1
)

type Region struct {
	Kind  RegionKind
	Depth uint8
	Start uint16
	End   uint16
}

func (r Region) SubString(s string) string {
	return s[r.Start:r.End]
}

type Regions []Region

type RegionFilter struct {
	ExactDepth   int
	MinimumDepth int        //ignored if ExactDepth is set
	Kind         RegionKind //ignored if zero
}

func (rs Regions) FilteredForEach(filter RegionFilter, fn func(r Region) error) error {

	for _, r := range rs {
		if filter.Kind != 0 && r.Kind != filter.Kind {
			continue
		}

		if filter.ExactDepth >= 0 && int(r.Depth) != filter.ExactDepth {
			continue
		}
		if filter.MinimumDepth >= 0 && int(r.Depth) < filter.MinimumDepth {
			continue
		}

		err := fn(r)
		if err != nil {
			return err
		}
	}
	return nil
}
