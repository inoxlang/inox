package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
)

func TestDatabaseIL(t *testing.T) {

	t.Run("Prop()", func(t *testing.T) {
		userPattern := NewExactObjectPattern(map[string]Pattern{}, nil)

		db := NewDatabaseIL(DatabaseILParams{
			Schema: NewExactObjectPattern(map[string]Pattern{
				"user": userPattern,
			}, nil),
			BaseURL: NewUrl("ldb://main/"),
		})

		expectedUser := userPattern.
			SymbolicValue().(*Object).
			Share(nil).(*Object).
			WithURL(NewUrl("ldb://main/user"))

		assert.Equal(t, expectedUser, db.Prop("user"))
	})

}

func TestGetValueAt(t *testing.T) {

	t.Run("collection element retrieval", func(t *testing.T) {
		collection := &_testCollection{List: NewList(ANY_STR)}
		userPattern := &TypePattern{val: collection}

		db := NewDatabaseIL(DatabaseILParams{
			Schema:  NewExactObjectPattern(map[string]Pattern{"users": userPattern}, nil),
			BaseURL: NewUrl("ldb://main/"),
		})

		elem, err := db.getValueAt("/users/100")
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, ANY_STR, elem)

		//the segment value following the collection should always be considered as an element key
		propName := "collection_prop"

		propVal := collection.Prop(propName) //check that the property is present
		if !assert.Equal(t, INT_1, propVal) {
			return
		}

		elem, err = db.getValueAt("/users/" + propName)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, ANY_STR, elem)
	})

	t.Run("indexable's element retrieval", func(t *testing.T) {
		t.Run("unknown length", func(t *testing.T) {
			userPattern := NewExactObjectPattern(map[string]Pattern{
				"list": NewListPatternOf(ANY_STR_PATTERN),
			}, nil)

			db := NewDatabaseIL(DatabaseILParams{
				Schema:  NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				BaseURL: NewUrl("ldb://main/"),
			})

			//index
			elem, err := db.getValueAt("/user/list/0")
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, ANY_STR, elem)
		})

		t.Run("known length", func(t *testing.T) {
			userPattern := NewExactObjectPattern(map[string]Pattern{
				"list": NewListPattern([]Pattern{ANY_STR_PATTERN}),
			}, nil)

			db := NewDatabaseIL(DatabaseILParams{
				Schema:  NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				BaseURL: NewUrl("ldb://main/"),
			})

			//index in range
			elem, err := db.getValueAt("/user/list/0")
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, ANY_STR, elem)

			//index out of range (negative)
			elem, err = db.getValueAt("/user/list/-1")
			if !assert.ErrorContains(t, err, fmtValueAtXDoesNotHavePropX("/user/list", "-1")) {
				return
			}
			assert.Nil(t, elem)

			//index out of range (positive)
			elem, err = db.getValueAt("/user/list/1")
			if !assert.ErrorContains(t, err, INDEX_IS_OUT_OF_RANGE) {
				return
			}
			assert.Nil(t, elem)
		})

	})

	t.Run("property retrieval", func(t *testing.T) {

		t.Run("base case", func(t *testing.T) {
			userPattern := NewExactObjectPattern(map[string]Pattern{
				"name":      &TypePattern{val: ANY_STR},
				"my_method": &TypePattern{val: NewInoxFunction(nil, nil, ANY)},
			}, nil)

			db := NewDatabaseIL(DatabaseILParams{
				Schema:  NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				BaseURL: NewUrl("ldb://main/"),
			})

			val, err := db.getValueAt("/user/name")
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, ANY_STR, val)
		})

		t.Run("retrieval of methods is not allowed", func(t *testing.T) {
			userPattern := NewExactObjectPattern(map[string]Pattern{
				"name":      &TypePattern{val: ANY_STR},
				"my_method": &TypePattern{val: NewInoxFunction(nil, nil, ANY)},
			}, nil)

			db := NewDatabaseIL(DatabaseILParams{
				Schema:  NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				BaseURL: NewUrl("ldb://main/"),
			})

			val, err := db.getValueAt("/user/my_method")
			if !assert.ErrorContains(t, err, fmtRetrievalOfMethodAtXIsNotAllowed("/user/my_method")) {
				return
			}
			assert.Nil(t, val)
		})

		t.Run("no properties", func(t *testing.T) {
			userPattern := NewExactObjectPattern(map[string]Pattern{
				"anyval": ANY_SERIALIZABLE_PATTERN,
			}, nil)

			db := NewDatabaseIL(DatabaseILParams{
				Schema:  NewExactObjectPattern(map[string]Pattern{"user": userPattern}, nil),
				BaseURL: NewUrl("ldb://main/"),
			})

			val, err := db.getValueAt("/user/anyval/prop")
			if !assert.ErrorContains(t, err, fmtValueAtXHasNoProperties("/user/anyval")) {
				return
			}
			assert.Nil(t, val)
		})
	})

}

var _ = Collection((*_testCollection)(nil))

type _testCollection struct {
	*List
	CollectionMixin
}

func (c *_testCollection) Prop(name string) Value {
	if name == "collection_prop" {
		return INT_1
	}
	return c.List.Prop(name)
}

func (c *_testCollection) PropertyNames() []string {
	return append(slices.Clone(c.List.PropertyNames()), "collection_prop")
}
