package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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

func (j *LifetimeJob) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	other, ok := v.(*LifetimeJob)
	if ok {
		return true
	}
	return j.subjectPattern.Test(other.subjectPattern, state)
}

func (j *LifetimeJob) WidestOfType() SymbolicValue {
	return &LifetimeJob{subjectPattern: ANY_PATTERN}
}

func (j *LifetimeJob) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (j *LifetimeJob) Prop(name string) SymbolicValue {
	method, ok := j.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, j))
	}
	return method
}

func (*LifetimeJob) PropertyNames() []string {
	return []string{}
}

func (r *LifetimeJob) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if IsAny(r.subjectPattern) {
		utils.Must(w.Write(utils.StringAsBytes("%lifetime-job")))
		return
	}
	utils.Must(w.Write(utils.StringAsBytes("%lifetime(\n")))
	r.subjectPattern.PrettyPrint(w, config, depth+1, parentIndentCount)
	utils.Must(w.Write(utils.StringAsBytes("\n)")))
}
