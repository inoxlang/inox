package commonfmt

type RegionInfo struct {
	Kind            string
	AssociatedValue any
	Start, End      int32 //byte indexes
}
