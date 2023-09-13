package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_JOB_IDLE_DURATION        = 10 * time.Millisecond
	JOB_SCHEDULING_TICK_INTERVAL = 50 * time.Microsecond
)

var (
	lifetimeJobSchedulerSpawned = atomic.Bool{}
	lifetimeJobsRegistrations   = make(chan *ValueLifetimeJobs, 1000)
	lifetimeJobsUnregistrations = make(chan *ValueLifetimeJobs, 1000)

	ErrLifetimeJobMetaValueShouldBeImmutable = errors.New("meta value of lifetime job should be immutable")
)

// A LifetimeJob represents a job associated with a value that runs while the value exists, this struct does not
// hold any state, see LifetimeJobInstance. LifetimeJob implements Value.
type LifetimeJob struct {
	meta                         Value   //immutable
	module                       *Module // module executed when running the job
	parentModule                 *Module
	subjectPattern               Pattern //nil if symbolicSubjectObjectPattern is set
	symbolicSubjectObjectPattern symbolic.Pattern
}

type LifetimeJobInstance struct {
	job    *LifetimeJob
	thread *LThread
}

func NewLifetimeJob(meta Value, subjectPattern Pattern, mod *Module, parentState *GlobalState) (*LifetimeJob, error) {
	if meta.IsMutable() {
		panic(ErrLifetimeJobMetaValueShouldBeImmutable)
	}
	return &LifetimeJob{
		meta:           meta,
		module:         mod,
		subjectPattern: subjectPattern,
		parentModule:   parentState.Module,
	}, nil
}

//func NewLifetimeJob(meta Value, subjectPattern Pattern, embeddedModChunk *parse.Chunk, parentState *GlobalState) (*LifetimeJob, error) {

// parsedChunk := &parse.ParsedChunk{
// 	Node:   embeddedModChunk,
// 	Source: parentState.Module.MainChunk.Source,
// }

// // manifest, err := evaluateLifetimeJobManifest(parsedChunk, parentState)
// // if err != nil {
// // 	return nil, err
// // }

// routineMod := &Module{
// 	MainChunk:        parsedChunk,
// 	ManifestTemplate: parsedChunk.Node.Manifest,
// 	ModuleKind:       LifetimeJobModule,
// }

// return &LifetimeJob{
// 	meta:           meta,
// 	module:         routineMod,
// 	subjectPattern: subjectPattern,
// 	parentModule:   parentState.Module,
// }, nil
//}

// Instantiate creates a instance of the job with a paused goroutine.
func (j *LifetimeJob) Instantiate(ctx *Context, self Value) (*LifetimeJobInstance, error) {
	spawnerState := ctx.GetClosestState()

	createLThreadPerm := LThreadPermission{Kind_: permkind.Create}

	if err := spawnerState.Ctx.CheckHasPermission(createLThreadPerm); err != nil {
		return nil, fmt.Errorf("lifetime job: following permission is required for running the job: %w", err)
	}

	manifest, _, _, err := j.module.PreInit(PreinitArgs{
		RunningState:          NewTreeWalkStateWithGlobal(spawnerState),
		ParentState:           spawnerState,
		AddDefaultPermissions: true,

		//TODO: should Project be set ?
	})

	if err != nil {
		return nil, err
	}

	permissions := utils.CopySlice(manifest.RequiredPermissions)
	permissions = append(permissions, createLThreadPerm)

	readGlobalPerm := GlobalVarPermission{Kind_: permkind.Read, Name: "*"}
	useGlobalPerm := GlobalVarPermission{Kind_: permkind.Use, Name: "*"}

	if !manifest.RequiresPermission(readGlobalPerm) {
		permissions = append(permissions, readGlobalPerm)
	}
	if !manifest.RequiresPermission(useGlobalPerm) {
		permissions = append(permissions, useGlobalPerm)
	}

	routineCtx := NewContext(ContextConfig{
		Kind:            DefaultContext,
		ParentContext:   ctx,
		Permissions:     permissions,
		Limits:          manifest.Limits,
		HostResolutions: manifest.HostResolutions,
	})

	for k, v := range ctx.GetNamedPatterns() {
		routineCtx.AddNamedPattern(k, v)
	}
	for k, v := range ctx.GetPatternNamespaces() {
		routineCtx.AddPatternNamespace(k, v)
	}

	// TODO: use tree walking interpreter for jobs not requiring performance ?
	// creating many VMs consumes at lot of memory.

	lthread, err := SpawnLThread(LthreadSpawnArgs{
		SpawnerState: spawnerState,
		LthreadCtx:   routineCtx,
		Globals:      spawnerState.Globals,
		Module:       j.module,
		Manifest:     manifest,
		UseBytecode:  j.module.Bytecode != nil,

		StartPaused: true,
		Self:        self,
	})

	if err != nil {
		return nil, fmt.Errorf("testing: %w", err)
	}

	return &LifetimeJobInstance{
		thread: lthread,
		job:    j,
	}, nil
}

func (j *LifetimeJob) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (j *LifetimeJob) Prop(ctx *Context, name string) Value {
	method, ok := j.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, j))
	}
	return method
}

func (*LifetimeJob) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*LifetimeJob) PropertyNames(ctx *Context) []string {
	return []string{}
}

// func evaluateLifetimeJobManifest(chunk *parse.ParsedChunk, parentState *GlobalState) (*Manifest, error) {
// 	createRoutinePerm := RoutinePermission{Kind_: permkind.Create}

// 	if err := parentState.Ctx.CheckHasPermission(createRoutinePerm); err != nil {
// 		return nil, fmt.Errorf("lifetime job: following permission is required for running the job: %w", err)
// 	}

// 	if chunk.Node.Manifest == nil {
// 		return &Manifest{
// 			RequiredPermissions: []Permission{createRoutinePerm},
// 		}, nil
// 	}

// 	objectLiteral := chunk.Node.Manifest.Object
// 	state := NewTreeWalkStateWithGlobal(parentState)

// 	manifestObj, err := evaluateManifestObjectNode(objectLiteral, ManifestEvaluationConfig{
// 		RunningState: state,
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	manifest, err := createManifest(manifestObj, manifestObjectConfig{
// 		addDefaultPermissions: true,
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	hasCreateRoutine := false

// 	for _, perm := range manifest.RequiredPermissions {
// 		if perm.Includes(createRoutinePerm) && createRoutinePerm.Includes(perm) {
// 			hasCreateRoutine = true
// 		}
// 		if err := parentState.Ctx.CheckHasPermission(perm); err != nil {
// 			return nil, fmt.Errorf("lifetime job: cannot allow permission: %w", err)
// 		}
// 	}

// 	if !hasCreateRoutine {
// 		manifest.RequiredPermissions = append(manifest.RequiredPermissions, createRoutinePerm)
// 	}
// 	return manifest, err
// }

type ValueLifetimeJobs struct {
	instances []*LifetimeJobInstance
	templates []*LifetimeJob
	idleTimes []time.Time
	started   bool
	self      Value
	lock      sync.Mutex
}

func NewValueLifetimeJobs(ctx *Context, self Value, jobs []*LifetimeJob) *ValueLifetimeJobs {
	//TODO: check that value is sharable

	valueJobs := &ValueLifetimeJobs{
		self:      self,
		idleTimes: make([]time.Time, len(jobs)),
		instances: make([]*LifetimeJobInstance, len(jobs)),
		templates: jobs,
	}

	for _, job := range jobs {
		symbolicSubject, err := self.(Serializable).ToSymbolicValue(ctx, map[uintptr]symbolic.SymbolicValue{})
		if err != nil {
			panic(err)
		}
		job.symbolicSubjectObjectPattern = utils.Must(symbolic.NewUncheckedExactValuePattern(symbolicSubject.(symbolic.Serializable)))
	}

	return valueJobs
}

func (jobs *ValueLifetimeJobs) InstantiateJobs(ctx *Context) error {
	jobs.lock.Lock()
	defer jobs.lock.Unlock()

	spawnLifetimeJobScheduler()
	now := time.Now()

	for i, job := range jobs.templates {
		instance, err := job.Instantiate(ctx, jobs.self)
		if err != nil {
			return fmt.Errorf("failed to instantiate (start) a lifetime job: %w", err)
		}
		jobs.instances[i] = instance
		jobs.idleTimes[i] = now
	}
	lifetimeJobsRegistrations <- jobs

	return nil
}

// Count returns the number of jobs at initialization.
func (jobs *ValueLifetimeJobs) Count() int {
	return len(jobs.templates)
}

func (jobs *ValueLifetimeJobs) Instances() []*LifetimeJobInstance {
	if jobs == nil {
		return nil
	}
	jobs.lock.Lock()
	defer jobs.lock.Unlock()
	return utils.CopySlice(jobs.instances)
}

func spawnLifetimeJobScheduler() {
	if !lifetimeJobSchedulerSpawned.CompareAndSwap(false, true) {
		return
	}

	scheduleJobsOfSingleValue := func(now time.Time, valueJobs *ValueLifetimeJobs) {
		valueJobs.lock.Lock()
		defer valueJobs.lock.Unlock()

		for i, job := range valueJobs.instances {
			if job == nil {
				continue
			}

			if job.thread.IsDone() {
				//TODO: cleanup
				continue
			}

			if !job.thread.IsPaused() {
				valueJobs.idleTimes[i] = now
				//TODO: find a way to pause
				continue
			}

			if !valueJobs.started || now.Sub(valueJobs.idleTimes[i]) > MAX_JOB_IDLE_DURATION {
				job.thread.ResumeAsync()
			}
		}
		valueJobs.started = true
	}

	ticks := time.Tick(JOB_SCHEDULING_TICK_INTERVAL)

	go func() {
		lifetimeJobs := map[*ValueLifetimeJobs]struct{}{}

		for {
			select {
			case valueJobs := <-lifetimeJobsRegistrations:
				if len(valueJobs.instances) == 0 {
					continue
				}

				lifetimeJobs[valueJobs] = struct{}{}
			case valueJobs := <-lifetimeJobsUnregistrations:
				delete(lifetimeJobs, valueJobs)

			case now := <-ticks:
				for valueJobs := range lifetimeJobs {
					scheduleJobsOfSingleValue(now, valueJobs)
				}
			}
		}
	}()

}
