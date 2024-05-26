package core

import "errors"

var (
	ErrReprOfMutableValueCanChange = errors.New("the representation of a mutable value can change")
)

func (err Error) IsMutable() bool {
	return false
}

func (n AstNode) IsMutable() bool {
	return false
}

func (t Token) IsMutable() bool {
	return false
}

func (Nil NilT) IsMutable() bool {
	return false
}

func (boolean Bool) IsMutable() bool {
	return false
}

func (r Rune) IsMutable() bool {
	return false
}

func (b Byte) IsMutable() bool {
	return false
}

func (i Int) IsMutable() bool {
	return false
}

func (f Float) IsMutable() bool {
	return false
}

func (s String) IsMutable() bool {
	return false
}

func (obj *Object) IsMutable() bool {
	return true
}

func (rec Record) IsMutable() bool {
	return false
}

func (dict *Dictionary) IsMutable() bool {
	return true
}

func (list KeyList) IsMutable() bool {
	return false
}

func (list *List) IsMutable() bool {
	return true
}

func (list *ValueList) IsMutable() bool {
	return true
}

func (list *NumberList[T]) IsMutable() bool {
	return true
}

func (list *BoolList) IsMutable() bool {
	return true
}

func (list *StringList) IsMutable() bool {
	return true
}

func (tuple *Tuple) IsMutable() bool {
	return false
}

func (tuple *OrderedPair) IsMutable() bool {
	return false
}

func (*Array) IsMutable() bool {
	return true
}

func (s *ModuleArgs) IsMutable() bool {
	return true
}

func (slice *RuneSlice) IsMutable() bool {
	return true
}

func (slice *ByteSlice) IsMutable() bool {
	return slice.Mutable()
}

func (goFunc *GoFunction) IsMutable() bool {
	return goFunc.kind != GoFunc
}

func (opt Option) IsMutable() bool {
	return opt.Value.IsMutable()
}

func (pth Path) IsMutable() bool {
	return false
}

func (patt PathPattern) IsMutable() bool {
	return false
}

func (u URL) IsMutable() bool {
	return false
}

func (host Host) IsMutable() bool {
	return false
}

func (scheme Scheme) IsMutable() bool {
	return false
}

func (patt HostPattern) IsMutable() bool {
	return false
}

func (addr EmailAddress) IsMutable() bool {
	return false
}

func (patt URLPattern) IsMutable() bool {
	return false
}

func (i Identifier) IsMutable() bool {
	return false
}

func (n PropertyName) IsMutable() bool {
	return false
}

func (p *LongValuePath) IsMutable() bool {
	return false
}

func (str CheckedString) IsMutable() bool {
	return false
}

func (count ByteCount) IsMutable() bool {
	return false
}

func (count LineCount) IsMutable() bool {
	return false
}

func (count RuneCount) IsMutable() bool {
	return false
}

func (rate ByteRate) IsMutable() bool {
	return false
}

func (f Frequency) IsMutable() bool {
	return false
}

func (d Duration) IsMutable() bool {
	return false
}

func (d Year) IsMutable() bool {
	return false
}

func (d Date) IsMutable() bool {
	return false
}

func (d DateTime) IsMutable() bool {
	return false
}

func (m FileMode) IsMutable() bool {
	return false
}

func (r RuneRange) IsMutable() bool {
	return false
}

func (r QuantityRange) IsMutable() bool {
	return false
}

func (r IntRange) IsMutable() bool {
	return false
}

func (r FloatRange) IsMutable() bool {
	return false
}

//patterns

func (pattern *ExactValuePattern) IsMutable() bool {
	return false
}

func (pattern *ExactStringPattern) IsMutable() bool {
	return false
}

func (pattern *TypePattern) IsMutable() bool {
	return false
}

func (pattern *DifferencePattern) IsMutable() bool {
	return false
}

func (pattern *OptionalPattern) IsMutable() bool {
	return false
}

func (pattern *FunctionPattern) IsMutable() bool {
	return false
}

func (patt *RegexPattern) IsMutable() bool {
	return false
}

func (patt *UnionPattern) IsMutable() bool {
	return false
}

func (patt *IntersectionPattern) IsMutable() bool {
	return false
}

func (patt *LengthCheckingStringPattern) IsMutable() bool {
	return false
}

func (patt *SequenceStringPattern) IsMutable() bool {
	return !patt.IsResolved()
}

func (patt *RepeatedPatternElement) IsMutable() bool {
	return !patt.IsResolved()
}

func (patt *UnionStringPattern) IsMutable() bool {
	return !patt.IsResolved()
}

func (patt DynamicStringPatternElement) IsMutable() bool {
	return false
}

func (patt *RuneRangeStringPattern) IsMutable() bool {
	return false
}

func (patt *IntRangePattern) IsMutable() bool {
	return false
}

func (patt *FloatRangePattern) IsMutable() bool {
	return false
}

func (patt *NamedSegmentPathPattern) IsMutable() bool {
	return false
}

func (patt *ObjectPattern) IsMutable() bool {
	return false
}

func (patt *RecordPattern) IsMutable() bool {
	return false
}

func (patt *ListPattern) IsMutable() bool {
	return false
}

func (patt *TuplePattern) IsMutable() bool {
	return false
}

func (patt *OptionPattern) IsMutable() bool {
	return false
}

func (patt *EventPattern) IsMutable() bool {
	return false
}

func (patt *MutationPattern) IsMutable() bool {
	return false
}

func (patt *ParserBasedPseudoPattern) IsMutable() bool {
	return false
}

func (patt *IntRangeStringPattern) IsMutable() bool {
	return false
}

func (patt *FloatRangeStringPattern) IsMutable() bool {
	return false
}

func (patt *PathStringPattern) IsMutable() bool {
	return false
}

func (*Reader) IsMutable() bool {
	return true
}

func (*Writer) IsMutable() bool {
	return true
}

func (mt Mimetype) IsMutable() bool {
	return false
}

func (i FileInfo) IsMutable() bool {
	return false
}

func (r *LThread) IsMutable() bool {
	return true
}

func (g *LThreadGroup) IsMutable() bool {
	return true
}

func (fn *InoxFunction) IsMutable() bool {
	return true
}

func (b *Bytecode) IsMutable() bool {
	return false
}

func (it *KeyFilteredIterator) IsMutable() bool {
	return true
}

func (it *ValueFilteredIterator) IsMutable() bool {
	return true
}

func (it *KeyValueFilteredIterator) IsMutable() bool {
	return true
}

func (it *ArrayIterator) IsMutable() bool {
	return true
}

func (it indexableIterator) IsMutable() bool {
	return true
}

func (it immutableSliceIterator[T]) IsMutable() bool {
	return true
}

func (it IntRangeIterator) IsMutable() bool {
	return true
}

func (it FloatRangeIterator) IsMutable() bool {
	return true
}

func (it RuneRangeIterator) IsMutable() bool {
	return true
}

func (it QuantityRangeIterator) IsMutable() bool {
	return true
}

func (it *PatternIterator) IsMutable() bool {
	return true
}

func (it indexedEntryIterator) IsMutable() bool {
	return true
}

func (it *IpropsIterator) IsMutable() bool {
	return true
}

func (it *EventSourceIterator) IsMutable() bool {
	return true
}

func (w *DirWalker) IsMutable() bool {
	return true
}

func (w *TreedataWalker) IsMutable() bool {
	return true
}

func (it *ValueListIterator) IsMutable() bool {
	return true
}

func (it *NumberListIterator[T]) IsMutable() bool {
	return true
}

func (it *BitSetIterator) IsMutable() bool {
	return true
}

func (it *StrListIterator) IsMutable() bool {
	return true
}

func (it *TupleIterator) IsMutable() bool {
	return true
}

func (t Type) IsMutable() bool {
	return true
}

func (tx *Transaction) IsMutable() bool {
	return true
}

func (r *RandomnessSource) IsMutable() bool {
	return true
}

func (m *Mapping) IsMutable() bool {
	return true
}

func (ns *PatternNamespace) IsMutable() bool {
	return false
}

func (port Port) IsMutable() bool {
	return false
}

func (u *Treedata) IsMutable() bool {
	return false
}

func (e TreedataHiearchyEntry) IsMutable() bool {
	return false
}

func (c *StringConcatenation) IsMutable() bool {
	//dirty hack: perform the concatenation now so that the *StringConcatenation
	//will never update any field in the future.
	c.GetOrBuildString()
	return false
}

func (c *BytesConcatenation) IsMutable() bool {
	return true
}

// func (s *TestSuite) IsMutable() bool {
// 	return false
// }

// func (c *TestCase) IsMutable() bool {
// 	return false
// }

// func (c *TestCaseResult) IsMutable() bool {
// 	return false
// }

func (e *Event) IsMutable() bool {
	return false
}

func (e ExecutedStep) IsMutable() bool {
	return true
}

func (w *GenericWatcher) IsMutable() bool {
	return true
}

func (w *PeriodicWatcher) IsMutable() bool {
	return true
}

func (m Mutation) IsMutable() bool {
	return false
}

func (w *joinedWatchers) IsMutable() bool {
	return true
}

func (w stoppedWatcher) IsMutable() bool {
	return true
}

func (*wrappedWatcherStream) IsMutable() bool {
	return true
}

func (*ElementsStream) IsMutable() bool {
	return true
}

func (*ReadableByteStream) IsMutable() bool {
	return true
}

func (*WritableByteStream) IsMutable() bool {
	return true
}

func (*ConfluenceStream) IsMutable() bool {
	return true
}

func (*RingBuffer) IsMutable() bool {
	return true
}

func (*DataChunk) IsMutable() bool {
	return true
}

func (*StaticCheckData) IsMutable() bool {
	return false
}

func (*SymbolicData) IsMutable() bool {
	return false
}

func (*Module) IsMutable() bool {
	return false
}

func (*GlobalState) IsMutable() bool {
	return true
}

func (*ValueHistory) IsMutable() bool {
	return true
}

func (*Secret) IsMutable() bool {
	return false
}

func (*SecretPattern) IsMutable() bool {
	return false
}

func (*MarkupPattern) IsMutable() bool {
	return false
}

func (*NonInterpretedMarkupElement) IsMutable() bool {
	return true
}

func (*ApiIL) IsMutable() bool {
	return true
}

func (ns *Namespace) IsMutable() bool {
	return ns.mutableEntries
}

func (*ModuleParamsPattern) IsMutable() bool {
	return false
}

// func (*CurrentTest) IsMutable() bool {
// 	return true
// }

// func (*TestedProgram) IsMutable() bool {
// 	return true
// }

func (ULID) IsMutable() bool {
	return false
}

func (UUIDv4) IsMutable() bool {
	return false
}
