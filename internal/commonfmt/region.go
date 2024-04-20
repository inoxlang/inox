package commonfmt

import (
	"io"

	"github.com/inoxlang/inox/internal/utils"
)

type RegionInfo struct {
	Kind            string
	AssociatedValue any
	Start, End      int32 //byte indexes
}

func (i RegionInfo) ByteLen() int32 {
	return i.End - i.Start
}

type RegionReplacement struct {
	Region        RegionInfo
	Before, After string
	String        string //ignored if ReFormat is set
	ReFormat      func(w io.Writer, value any) error
}

func Reformat(w io.Writer, text string, replacements []RegionReplacement) error {
	originalTextIndex := int32(0)
	for _, replacement := range replacements {
		var err error
		if replacement.Region.Start != originalTextIndex {
			before := text[originalTextIndex:replacement.Region.Start]

			_, err = w.Write(utils.StringAsBytes(before))
			if err != nil {
				return err
			}
		}

		_, err = w.Write(utils.StringAsBytes(replacement.Before))
		if err != nil {
			return err
		}

		if replacement.ReFormat != nil {
			err = replacement.ReFormat(w, replacement.Region.AssociatedValue)
		} else {
			_, err = w.Write(utils.StringAsBytes(replacement.String))
		}
		if err != nil {
			return err
		}

		_, err = w.Write(utils.StringAsBytes(replacement.After))
		if err != nil {
			return err
		}

		originalTextIndex += replacement.Region.End
	}
	if originalTextIndex < int32(len(text)) {
		_, err := w.Write(utils.StringAsBytes(text[originalTextIndex:]))
		return err
	}
	return nil
}
