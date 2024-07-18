package symbolic

func (any Any) IsMutable() bool {
	return true
}

func (Never) IsMutable() bool {
	return true
}

func (o Opaque) IsMutable() bool {
	return true
}

func (*AnySerializable) IsMutable() bool {
	return true
}

func (any AnyPattern) IsMutable() bool {
	return false
}

func (any AnySerializablePattern) IsMutable() bool {
	return false
}

func (any AnyStringPattern) IsMutable() bool {
	return false
}

func (any AnyBytesLike) IsMutable() bool {
	return true
}

func (any Function) IsMutable() bool {
	return false
}

func (any AnyReadable) IsMutable() bool {
	return true
}

func (any AnyWritable) IsMutable() bool {
	return true
}

func (any AnyIterable) IsMutable() bool {
	return true
}

func (any AnySerializableIterable) IsMutable() bool {
	return true
}

func (any AnyContainer) IsMutable() bool {
	return true
}

func (any AnyWatchable) IsMutable() bool {
	return true
}

func (any AnyIndexable) IsMutable() bool {
	return true
}

func (*AnySequenceOf) IsMutable() bool {
	return true
}

func (any AnyWalkable) IsMutable() bool {
	return true
}

func (any AnyStringLike) IsMutable() bool {
	return false
}

func (any AnyResourceName) IsMutable() bool {
	return false
}

func (any AnyStreamSource) IsMutable() bool {
	return true
}

func (any AnyStreamSink) IsMutable() bool {
	return true
}

func (any AnyIntegral) IsMutable() bool {
	return false
}

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

func (*Array) IsMutable() bool {
	return true
}

func (list *List) IsMutable() bool {
	return true
}

func (tuple *Tuple) IsMutable() bool {
	return false
}

func (tuple *OrderedPair) IsMutable() bool {
	return false
}

func (s *ModuleArgs) IsMutable() bool {
	return true
}

func (slice *RuneSlice) IsMutable() bool {
	return true
}

func (slice *ByteSlice) IsMutable() bool {
	return true
	//return slice.Mutable()
}

func (goFunc *GoFunction) IsMutable() bool {
	return goFunc.kind != GoFunc
}

func (opt Option) IsMutable() bool {
	return true
	//return opt.Value.IsMutable()
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

func (p PropertyName) IsMutable() bool {
	return false
}

func (p LongValuePath) IsMutable() bool {
	return false
}

func (p AnyValuePath) IsMutable() bool {
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
	return false
}

func (patt *ParserBasedPattern) IsMutable() bool {
	return false
}

func (patt *IntRangeStringPattern) IsMutable() bool {
	return false
}

func (patt *FloatRangeStringPattern) IsMutable() bool {
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

func (it *Iterator) IsMutable() bool {
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

func (e *Event) IsMutable() bool {
	return false
}

func (e *EventSource) IsMutable() bool {
	return true
}

func (w *Walker) IsMutable() bool {
	return true
}

func (s *ExecutedStep) IsMutable() bool {
	return true
}

func (c *AnyProtocolClient) IsMutable() bool {
	return true
}

func (mv *Multivalue) IsMutable() bool {
	for _, val := range mv.values {
		if val.IsMutable() {
			return true
		}
	}
	return false
}

func (rv *RunTimeValue) IsMutable() bool {
	return rv.super.IsMutable()
}

func (w *Watcher) IsMutable() bool {
	return true
}

func (w Mutation) IsMutable() bool {
	return false
}

func (s *ReadableStream) IsMutable() bool {
	return true
}

func (s *WritableStream) IsMutable() bool {
	return true
}

func (r *RingBuffer) IsMutable() bool {
	return true
}

func (c *DataChunk) IsMutable() bool {
	return true
}

func (d *StaticCheckData) IsMutable() bool {
	return false
}

func (d *Data) IsMutable() bool {
	return false
}

func (m *Module) IsMutable() bool {
	return false
}

func (s *GlobalState) IsMutable() bool {
	return true
}

func (f *AnyFormat) IsMutable() bool {
	return false
}

func (*ValueHistory) IsMutable() bool {
	return true
}

func (*Snapshot) IsMutable() bool {
	return false
}

func (*AnyInMemorySnapshotable) IsMutable() bool {
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
	return false
}

func (*AnyMarkupNode) IsMutable() bool {
	return true
}

func (*ApiIL) IsMutable() bool {
	return true
}

func (ns *Namespace) IsMutable() bool {
	if ns.checkMutability {
		return ns.mutableEntries
	}
	return true
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

func (*ULID) IsMutable() bool {
	return false
}

func (*UUIDv4) IsMutable() bool {
	return false
}

func (*Bytecode) IsMutable() bool {
	return false
}
