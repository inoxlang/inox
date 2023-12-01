package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTreedata(t *testing.T) {

	t.Run("getEntryAtIndexes", func(t *testing.T) {
		treedata := &Treedata{
			Root: Int(1),
			HiearchyEntries: []TreedataHiearchyEntry{
				{
					Value: Int(2),
					Children: []TreedataHiearchyEntry{
						{Value: Int(3)},
						{
							Value: Int(4),
							Children: []TreedataHiearchyEntry{
								{
									Value: Int(5),
								},
							},
						},
					},
				},
				{Value: Int(6)},
			},
		}

		entry, ok := treedata.getEntryAtIndexes(0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(2), entry.Value)

		entry, ok = treedata.getEntryAtIndexes(0, 0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(3), entry.Value)

		entry, ok = treedata.getEntryAtIndexes(0, 1)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(4), entry.Value)

		entry, ok = treedata.getEntryAtIndexes(0, 1, 0)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(5), entry.Value)

		entry, ok = treedata.getEntryAtIndexes(1)
		if !assert.True(t, ok) {
			return
		}
		assert.Equal(t, Int(6), entry.Value)
	})

}
