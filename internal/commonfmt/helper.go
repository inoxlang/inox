package commonfmt

import (
	"bytes"
	"slices"
)

// A Helper is a stateful formatting helper that can be used to format a single message.
// Calling Consume() returns the result and resets the helper. Helper is not thread safe.
type Helper struct {
	messageBuff, tempRegionBuff *bytes.Buffer
	regions                     []RegionInfo
}

func NewHelper() *Helper {
	return &Helper{
		messageBuff:    &bytes.Buffer{},
		tempRegionBuff: &bytes.Buffer{},
	}
}

func (h *Helper) AppendString(s string) int32 {
	n, _ := h.messageBuff.WriteString(s)
	return int32(n)
}

type RegionParams struct {
	Kind            string
	AssociatedValue any
	Format          func(tempRegionWriter *bytes.Buffer, value any) error
}

func (h *Helper) AppendRegion(params RegionParams) error {
	startIndex := int32(h.messageBuff.Len())
	defer h.tempRegionBuff.Reset()

	err := params.Format(h.tempRegionBuff, params.AssociatedValue)
	if err != nil {
		return err
	}

	h.messageBuff.Write(h.tempRegionBuff.Bytes())

	endIndex := int32(h.messageBuff.Len())

	h.regions = append(h.regions, RegionInfo{
		Kind:            params.Kind,
		AssociatedValue: params.AssociatedValue,
		Start:           startIndex,
		End:             endIndex,
	})

	return nil
}

func (h *Helper) Consume() (string, []RegionInfo) {
	regions := slices.Clone(h.regions)
	h.regions = h.regions[:0]
	s := h.messageBuff.String()
	h.messageBuff.Reset()
	h.tempRegionBuff.Reset()
	return s, regions
}
