package core

var (
	_ = []PotentialSystem{(*Object)(nil)}
	_ = []SystemPart{(*Object)(nil)}
)

type PotentialSystem interface {
	PotentiallySharable

	// SystemParts returns all components & subsystems of the system, the returned slice should not be modified.
	SystemParts() []SystemPart

	// LifetimeJobs returns the lifetime jobs of the value, the returned slice shoyld not be modified.
	LifetimeJobs() *ValueLifetimeJobs
}

type SystemPart interface {
	SystemGraphNodeValue
	AttachToSystem(s PotentialSystem) error
	DetachFromSystem() error
	System() (PotentialSystem, error)
}

func IsSystem(v Value) bool {
	potentialSys, ok := v.(PotentialSystem)
	if !ok {
		return false
	}
	return potentialSys.LifetimeJobs().Count() != 0 || len(potentialSys.SystemParts()) != 0
}
