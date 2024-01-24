package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	THREAD_PROPNAMES            = []string{"add"}
	THREAD_ADD_METHOD_ARG_NAMES = []string{"message"}

	ANY_THREAD = newThread(symbolic.ANY_OBJECT_PATTERN)

	_ = []symbolic.Iterable{(*MessageThread)(nil)}
	_ = []symbolic.Collection{(*MessageThread)(nil)}
	_ = []symbolic.Serializable{(*MessageThread)(nil)}
	_ = []symbolic.PotentiallySharable{(*MessageThread)(nil)}
	_ = []symbolic.UrlHolder{(*MessageThread)(nil)}

	_ = []symbolic.PotentiallyConcretizable{(*MessageThreadPattern)(nil)}
	_ = []symbolic.MigrationInitialValueCapablePattern{(*MessageThreadPattern)(nil)}
)

type MessageThread struct {
	elementPattern *symbolic.ObjectPattern
	element        *symbolic.Object

	url *symbolic.URL

	addMethodParamsCache *[]symbolic.Value

	symbolic.CollectionMixin
	symbolic.SerializableMixin
}

func newThread(elementPattern *symbolic.ObjectPattern) *MessageThread {
	t := &MessageThread{
		elementPattern: elementPattern,
		element:        elementPattern.SymbolicValue().(*symbolic.Object),
	}
	t.addMethodParamsCache = &[]symbolic.Value{t.element}
	return t
}

func (t *MessageThread) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherThread, ok := v.(*MessageThread)
	return ok && t.elementPattern.Test(otherThread.elementPattern, symbolic.RecTestCallState{})
}

func (t *MessageThread) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(t.Add), true
	}
	return nil, false
}

func (t *MessageThread) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*MessageThread) PropertyNames() []string {
	return THREAD_PROPNAMES
}

func (t *MessageThread) IsSharable() (bool, string) {
	return true, ""
}

func (t *MessageThread) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	return t
}

func (t *MessageThread) IsShared() bool {
	return true
}

func (t *MessageThread) WithURL(url *symbolic.URL) symbolic.UrlHolder {
	copy := *t
	copy.url = url

	elementURL := copy.url.WithAdditionalPathPatternSegment("*")
	copy.element = t.element.WithURL(elementURL).(*symbolic.Object)
	return &copy
}

func (s *MessageThread) URL() (*symbolic.URL, bool) {
	if s.url != nil {
		return s.url, true
	}
	return nil, false
}

func (t *MessageThread) Contains(value symbolic.Serializable) (yes bool, possible bool) {
	if !t.element.Test(value, symbolic.RecTestCallState{}) {
		return
	}

	possible = true
	return
}

func (t *MessageThread) Add(ctx *symbolic.Context, elem *symbolic.Object) {
	ctx.SetSymbolicGoFunctionParameters(t.addMethodParamsCache, THREAD_ADD_METHOD_ARG_NAMES)
}

func (*MessageThread) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("thread")
}

func (t *MessageThread) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (*MessageThread) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*MessageThread) WidestOfType() symbolic.Value {
	return ANY_THREAD
}
