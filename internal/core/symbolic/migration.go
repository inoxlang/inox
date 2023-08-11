package symbolic

type MigrationOp interface {
	GetPseudoPath() string
}

type MigrationMixin struct {
	PseudoPath string
}

func (m MigrationMixin) GetPseudoPath() string {
	return m.PseudoPath
}

type ReplacementMigrationOp struct {
	Current, Next Pattern
	MigrationMixin
}

type RemovalMigrationOp struct {
	Value Pattern
	MigrationMixin
}

type NillableInitializationMigrationOp struct {
	Value Pattern
	MigrationMixin
}

type InclusionMigrationOp struct {
	Value    Pattern
	Optional bool
	MigrationMixin
}

type MigrationInitialValueCapablePattern interface {
	//MigrationInitialValue returns the initial value accepted by the pattern for initialization.
	MigrationInitialValue() (Serializable, bool)
}
