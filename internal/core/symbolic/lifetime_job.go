package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

// A LifetimeJob represents a symbolic LifetimeJob.
type LifetimeJob struct {
	subjectPattern Pattern

	UnassignablePropsMixin
	SerializableMixin
}

func NewLifetimeJob(subjectPattern Pattern) *LifetimeJob {
	return &LifetimeJob{subjectPattern: subjectPattern}
}

func (j *LifetimeJob) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*LifetimeJob)
	if ok {
		return true
	}
	return j.subjectPattern.Test(other.subjectPattern, state)
}

func (j *LifetimeJob) WidestOfType() Value {
	return &LifetimeJob{subjectPattern: ANY_PATTERN}
}

func (j *LifetimeJob) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (j *LifetimeJob) Prop(name string) Value {
	method, ok := j.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, j))
	}
	return method
}

func (*LifetimeJob) PropertyNames() []string {
	return []string{}
}

func (r *LifetimeJob) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if IsAny(r.subjectPattern) {
		w.WriteName("lifetime-job")
		return
	}
	w.WriteName("lifetime(\n")
	r.subjectPattern.PrettyPrint(w.IncrDepth(), config)
	w.WriteString("\n)")
}
