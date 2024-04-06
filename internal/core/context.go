package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	CTX_DONE_MICROTASK_CALLS_TIMEOUT = 5 * time.Millisecond
	DEFAULT_IWD                      = Path("/")
)

var (
	ErrBothCtxFilesystemArgsProvided            = errors.New("invalid arguments: both .CreateFilesystem & .Filesystem provided")
	ErrBothParentCtxArgsProvided                = errors.New("invalid arguments: both .ParentContext & .ParentStdLibContext provided")
	ErrInitialWorkingDirProvidedWithoutFS       = errors.New("invalid arguments: .InitialWorkingDirectory is set but no filesystem is provided")
	ErrAdditionalParentCtxSetButNoGoroutine     = errors.New("invalid arguments: .AdditionalParentContext is set but DoNotSpawnDoneGoroutine is true")
	ErrImpossibleToDeterminateInitialWorkingDir = errors.New("impossible to determinate initial working directory")

	ErrNonExistingNamedPattern                 = errors.New("non existing named pattern")
	ErrNotUniquePatternDefinition              = errors.New("cannot register a pattern more than once")
	ErrNotUniquePatternNamespaceDefinition     = errors.New("cannot register a pattern namespace more than once")
	ErrNotUniqueHostDefinitionDefinition       = errors.New("cannot set host definition data more than once")
	ErrNotUniqueProtocolClient                 = errors.New("client already defined")
	ErrCannotProvideLimitTokensForChildContext = errors.New("limit tokens cannot be set in new context's config if it is a child")
	ErrNoAssociatedState                       = errors.New("context has no associated state")
	ErrAlreadyAssociatedState                  = errors.New("context already has an associated state")
	ErrNotSharableNorClonableUserDataValue     = errors.New("attempt to set a user data entry with a value that is not sharable nor clonable")
	ErrDoubleUserDataDefinition                = errors.New("cannot define a user data entry more than once")
	ErrTypeExtensionAlreadyRegistered          = errors.New("type extension is already registered")

	ErrLimitNotPresentInContext   = errors.New("limit not present in context")
	ErrOnDoneMicrotasksNotAllowed = errors.New("'on done' microtasks are not allowed")

	nopLogger = zerolog.Nop()
)

// A Context is analogous to contexts provided by the stdlib's `context` package.
// When the context is cancelled all descendant contexts are cancelled as well.
// All *GlobalState instances have a context. An Inox Context has several roles:
// - It stores named patterns, pattern namespaces and other module's data.
// - It is called by the runtime and native functions to check permissions and enforce limits.
// - It has a reference to the current transaction.
// - During graceful teardown it calls functions registered with OnGracefulTearDown.
// - After cancellation it executes microtasks registered with OnDone.
type Context struct {
	context.Context

	kind      ContextKind
	lock      sync.RWMutex
	parentCtx *Context

	// associated state, it is set at the creation of the associated state.
	state                   *GlobalState
	fs                      afs.Filesystem
	initialWorkingDirectory Path

	currentTx     *Transaction
	currentTxLock sync.Mutex

	longLived                    atomic.Bool
	done                         atomic.Bool
	cancel                       context.CancelFunc
	gracefulTearDownCallLocation atomic.Value //nil or string

	onGracefulTearDownTasks []GracefulTearDownTaskFn
	gracefulTearDownStatus  atomic.Int64 //see GracefulTeardownStatus

	tearedDown               atomic.Bool //true when the context is done and the 'done' microtasks have been called.
	disallowOnDoneMicrotasks bool
	onDoneMicrotasks         []ContextDoneMicrotaskFn

	//permissions & limits
	grantedPermissions   []Permission
	forbiddenPermissions []Permission
	limits               []Limit
	limiters             map[string]*limiter //the map is not changed after context creation

	//values
	namedPatterns       map[string]Pattern
	patternNamespaces   map[string]*PatternNamespace
	urlProtocolClients  map[URL]ProtocolClient
	hostProtocolClients map[Host]ProtocolClient
	hostDefinitions     map[Host]Value
	userData            map[Path]Value
	typeExtensions      []*TypeExtension

	executionStartTime time.Time

	tempDir           Path //directory for storing temporary files, defaults to a random directory in /tmp
	waitConfirmPrompt WaitConfirmPrompt
}

type ContextKind int

const (
	DefaultContext ContextKind = iota
	TestingContext
)

type GracefulTeardownStatus int32

const (
	NeverStartedGracefulTeardown GracefulTeardownStatus = iota
	GracefullyTearingDown
	GracefullyTearedDown
	GracefullyTearedDownWithErrors       //errors during teardown
	GracefullyTearedDownWithCancellation //context cancellation before the end of the teardown
)

// A GracefulTearDownTaskFn should ideally run for a relative short time (less than 500ms),
// the passed context is the context the microtask was registered to.
type GracefulTearDownTaskFn func(ctx *Context) error

// A ContextDoneMicrotaskFn should run for a short time (less than 1ms),
// the calling context should not be access because it is locked.
type ContextDoneMicrotaskFn func(timeoutCtx context.Context, teardownStatus GracefulTeardownStatus) error

type ContextConfig struct {
	Kind                    ContextKind
	Permissions             []Permission
	ForbiddenPermissions    []Permission
	DoNotCheckDatabasePerms bool

	//If (cpu time limit is not present) AND (parent context has it) then the limit is inherited.
	//The depletion of total limits' tokens for the created context starts when the associated state is set.
	Limits []Limit

	HostDefinitions     map[Host]Value
	TypeExtensions      []*TypeExtension
	OwnedDatabases      []DatabaseConfig
	ParentContext       *Context
	ParentStdLibContext context.Context //should not be set if ParentContext is set
	LimitTokens         map[string]int64

	//An optional context whose cancellation causes a call to Context.CancelGracefully().
	//This field should not be set if DoNotSpawnDoneGoroutine is true.
	AdditionalParentContext context.Context

	Filesystem       afs.Filesystem
	CreateFilesystem func(ctx *Context) (afs.Filesystem, error) //should not be set if Filesystem is set

	// If false the context's filesystem is the result of WithSecondaryContextIfPossible(ContextConfig.Filesystem),
	// else the context's filesystem is ContextConfig.Filesystem.
	DoNotSetFilesystemContext bool
	InitialWorkingDirectory   Path //if not set defaults to '/'

	// If false a goroutine is created to tear down the context after it is done.
	// if true IsDone() will always return false until CancelGracefully is called.
	DoNotSpawnDoneGoroutine bool

	WaitConfirmPrompt WaitConfirmPrompt
}

type WaitConfirmPrompt func(msg string, accepted []string) (bool, error)

// If .ParentContext is set Check verifies that:
// - the parent have at least the permissions required by the child
// - the parent have less restrictive limits than the child
// - no host definition of the parent is overriden
func (c ContextConfig) Check() (firstErr error, ok bool) {
	if c.ParentContext == nil {
		return nil, true
	}

outer_loop:
	for _, perm := range c.Permissions {

		dbPerm, ok := perm.(DatabasePermission)
		if ok {
			if c.DoNotCheckDatabasePerms {
				continue outer_loop
			}

			for _, dbConfig := range c.OwnedDatabases {
				if dbConfig.IsPermissionForThisDB(dbPerm) {
					continue outer_loop
				}
			}
		}

		if err := c.ParentContext.CheckHasPermission(perm); err != nil {
			return fmt.Errorf("parent of context should at least have permissions of its child: %w", err), false
		}
	}

	for _, limit := range c.Limits {
		if parentLimiter, ok := c.ParentContext.limiters[limit.Name]; ok && !parentLimiter.limit.LessOrAsRestrictiveAs(limit) {
			return fmt.Errorf("parent of context should have less restrictive limits than its child: limit '%s'", limit.Name), false
		}
	}

	for host := range c.ParentContext.GetAllHostDefinitions() {
		if _, ok := c.HostDefinitions[host]; ok {
			return fmt.Errorf("the host '%s' is predefined by child context but is already defined by the parent context", host), false
		}
	}

	return nil, true
}

// NewContextWithEmptyState creates a context & an empty state,
// out is used as the state's output (or io.Discard if nil),
// OutputFieldsInitialized is set to true.
func NewContextWithEmptyState(config ContextConfig, out io.Writer) *Context {
	ctx := NewContext(config)
	state := NewGlobalState(ctx)

	if out == nil {
		state.Out = io.Discard
		state.Logger = zerolog.Nop()
	} else {
		state.Out = out
		state.Logger = zerolog.New(out)
	}

	state.OutputFieldsInitialized.Store(true)
	return ctx
}

// NewContext creates a new context, if a parent context is provided the embedded context.Context will be
// context.WithCancel(parentContext), otherwise it will be context.WithCancel(context.Background()).
func NewContext(config ContextConfig) *Context {

	var (
		limiters                map[string]*limiter
		stdlibCtx               context.Context
		cancel                  context.CancelFunc
		filesystem              afs.Filesystem = config.Filesystem
		initialWorkingDirectory Path           = config.InitialWorkingDirectory
		ctx                                    = &Context{} //the context is initialized later in the function but we need the address
		additionalParentCtx                    = config.AdditionalParentContext
	)

	if config.CreateFilesystem != nil {
		if filesystem != nil {
			panic(ErrBothCtxFilesystemArgsProvided)
		}

		fs, err := config.CreateFilesystem(ctx)
		if err != nil {
			panic(err)
		}
		filesystem = fs
	}

	if initialWorkingDirectory != "" && filesystem == nil && config.ParentContext == nil {
		panic(ErrInitialWorkingDirProvidedWithoutFS)
	}

	if additionalParentCtx != nil && config.DoNotSpawnDoneGoroutine {
		panic(ErrAdditionalParentCtxSetButNoGoroutine)
	}

	//create limiters
	limiters = make(map[string]*limiter)
	var parentContextLimiters map[string]*limiter

	if config.ParentContext != nil {
		if config.ParentStdLibContext != nil {
			panic(ErrBothParentCtxArgsProvided)
		}
		parentContextLimiters = config.ParentContext.limiters
	}

	for _, l := range config.Limits {
		_, alreadyExist := limiters[l.Name]
		if alreadyExist {
			//TODO: use logger
			panic(fmt.Errorf("context creation: duplicate limit '%s'", l.Name))
		}

		if limiter, ok := parentContextLimiters[l.Name]; ok {
			limiters[l.Name] = limiter.Child()
			continue
		}

		var fillRate int64 = 1

		switch l.Kind {
		case ByteRateLimit, FrequencyLimit:
			fillRate = l.Value
		case TotalLimit:
			fillRate = 0 //incrementation/decrementation is handled by the limit's DecrementFn
		}

		var cap int64 = l.Value
		initialAvail, ok := config.LimitTokens[l.Name]
		if !ok {
			initialAvail = -1 //capacity
		}

		limiters[l.Name] = &limiter{
			limit: l,
			bucket: newBucket(tokenBucketConfig{
				cap:                          cap,
				initialAvail:                 initialAvail,
				fillRate:                     fillRate,
				depleteFn:                    l.DecrementFn,
				cancelContextOnNegativeCount: l.Kind == TotalLimit,
			}),
		}
	}

	hostDefinitions := map[Host]Value{}
	maps.Copy(hostDefinitions, config.HostDefinitions)
	parentCtx := config.ParentContext

	if parentCtx == nil {
		parentStdLibContext := config.ParentStdLibContext
		if parentStdLibContext == nil {
			parentStdLibContext = context.Background()
		}

		stdlibCtx, cancel = context.WithCancel(parentStdLibContext)

		if initialWorkingDirectory == "" {
			initialWorkingDirectory = DEFAULT_IWD
		}
	} else {
		//if a parent context is passed we check that the parent has all the required permissions
		if err, ok := config.Check(); !ok {
			panic(fmt.Errorf("failed to create context: invalid context configuration: %w", err))
		}

		if config.LimitTokens != nil {
			panic(ErrCannotProvideLimitTokensForChildContext)
		}

		//inherit limits from parent
		for _, parentLimiter := range parentCtx.limiters {
			limitName := parentLimiter.limit.Name
			if _, ok := limiters[limitName]; !ok {
				limiters[limitName] = parentLimiter.Child()
			}
		}

		//inherit host definitions from parent
		parentHostDefinitions := parentCtx.GetAllHostDefinitions()
		for host, data := range parentHostDefinitions {
			if _, ok := hostDefinitions[host]; ok {
				panic(ErrUnreachable)
			}
			hostDefinitions[host] = data
		}

		stdlibCtx, cancel = context.WithCancel(parentCtx)

		if filesystem == nil {
			filesystem = parentCtx.fs
			initialWorkingDirectory = parentCtx.initialWorkingDirectory
		}
	}

	limits := make([]Limit, 0)
	for _, limiter := range limiters {
		limits = append(limits, limiter.limit)
	}

	actualFilesystem := filesystem

	if !config.DoNotSetFilesystemContext {
		actualFilesystem = WithSecondaryContextIfPossible(ctx, filesystem)
	}

	if initialWorkingDirectory == "" {
		if _, ok := filesystem.(afs.OsFS); ok {
			wd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			initialWorkingDirectory = DirPathFrom(wd)
		} else {
			initialWorkingDirectory = DEFAULT_IWD
		}
	}

	*ctx = Context{
		kind:                    config.Kind,
		Context:                 stdlibCtx,
		cancel:                  cancel,
		parentCtx:               parentCtx,
		fs:                      actualFilesystem,
		initialWorkingDirectory: initialWorkingDirectory,
		executionStartTime:      time.Now(),
		grantedPermissions:      slices.Clone(config.Permissions),
		forbiddenPermissions:    slices.Clone(config.ForbiddenPermissions),
		limits:                  limits,
		limiters:                limiters,
		namedPatterns:           map[string]Pattern{},
		patternNamespaces:       map[string]*PatternNamespace{},
		urlProtocolClients:      map[URL]ProtocolClient{},
		hostProtocolClients:     map[Host]ProtocolClient{},
		hostDefinitions:         hostDefinitions,
		userData:                map[Path]Value{},
		typeExtensions:          slices.Clone(config.TypeExtensions),

		waitConfirmPrompt: config.WaitConfirmPrompt,
	}

	for _, limiter := range limiters {
		limiter.SetContextIfNotChild(ctx)
	}

	ctx.gracefulTearDownStatus.Store(int64(NeverStartedGracefulTeardown))

	if config.DoNotSpawnDoneGoroutine {
		ctx.disallowOnDoneMicrotasks = true
		return ctx
	}

	//tear down
	go func() {
		if additionalParentCtx == nil {
			<-ctx.Done()
		} else {
			select {
			case <-additionalParentCtx.Done():
				go func() {
					defer utils.Recover() //recover even if CancelGracefully() should never panic.
					ctx.CancelGracefully()
				}()
				<-ctx.Done()
			case <-ctx.Done():
			}
		}
		ctx.done.Store(true)

		if ctx.GracefulTearDownStatus() == GracefullyTearingDown {
			ctx.gracefulTearDownStatus.Store(int64(GracefullyTearedDownWithCancellation))
		}

		defer ctx.tearedDown.Store(true)

		ctx.lock.Lock()
		defer ctx.lock.Unlock()

		logger := ctx.loggerNoLock()

		//ctx.finished = true

		for _, limiter := range ctx.limiters {
			limiter.Destroy()
		}

		//release acquired resources
		//TODO

		//rollback transaction (the rollback will be ignored if the transaction is finished)
		ctx.currentTxLock.Lock()
		tx := ctx.currentTx
		ctx.currentTxLock.Unlock()

		if tx != nil {
			tx.Rollback(ctx)
		}

		//call microtasks
		deadlineCtx, cancelTimeoutCtx := context.WithTimeout(context.Background(), CTX_DONE_MICROTASK_CALLS_TIMEOUT)
		defer func() {
			cancelTimeoutCtx()
			ctx.onDoneMicrotasks = nil
		}()

	microtasks:
		for _, microtaskFn := range ctx.onDoneMicrotasks {
			func() {
				defer func() {
					if e := recover(); e != nil {
						defer utils.Recover()
						err := fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), string(debug.Stack()))
						logger.Err(err).Msg("error while calling a context done microtask")
					}
				}()

				err := microtaskFn(deadlineCtx, ctx.GracefulTearDownStatus())
				if err != nil {
					logger.Err(err).Msg("error returned by a context done microtask")
				}
			}()

			select {
			case <-deadlineCtx.Done():
				break microtasks
			default:
			}
		}
	}()

	// no additional code is expected here
	// because this section is unreachable if DoNotSpawnDoneGoroutine is true.

	return ctx
}

func (ctx *Context) makeDoneContextError() error {
	if location, ok := ctx.gracefulTearDownCallLocation.Load().(string); ok {
		return fmt.Errorf("done context: %w\n((CancelGracefully() was called here:\n%s))", ctx.Err(), location)
	}
	return fmt.Errorf("done context: %w", ctx.Err())
}

func (ctx *Context) assertNotDone() {
	if ctx.done.Load() {
		panic(ctx.makeDoneContextError())
	}
}

func (ctx *Context) IsDone() bool {
	return ctx.done.Load()
}

func (ctx *Context) IsDoneSlowCheck() bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (ctx *Context) OnDone(microtask ContextDoneMicrotaskFn) {
	if ctx.disallowOnDoneMicrotasks {
		panic(ErrOnDoneMicrotasksNotAllowed)
	}
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	ctx.onDoneMicrotasks = append(ctx.onDoneMicrotasks, microtask)
}

func (ctx *Context) OnGracefulTearDown(task GracefulTearDownTaskFn) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.onGracefulTearDownTasks = append(ctx.onGracefulTearDownTasks, task)
}

func (ctx *Context) GracefulTearDownStatus() GracefulTeardownStatus {
	return GracefulTeardownStatus(ctx.gracefulTearDownStatus.Load())
}

// IsTearedDown returns true if the context is done and the 'done' microtasks have been called.
func (ctx *Context) IsTearedDown() bool {
	return ctx.tearedDown.Load()
}

func (ctx *Context) InefficientlyWaitUntilTearedDown(timeout time.Duration) bool {
	utils.InefficientlyWaitUntilTrue(&ctx.tearedDown, timeout)
	return ctx.IsTearedDown()
}

func (ctx *Context) MustGetClosestState() *GlobalState {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	ctx.assertNotDone()

	return ctx.mustGetClosestStateNoLock()
}

// GetState returns the state associated with the context, the boolean is false if the state is not set.
// To get the closest state GetClosestState() should be used.
func (ctx *Context) GetState() (*GlobalState, bool) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	ctx.assertNotDone()

	if ctx.state == nil {
		return nil, false
	}

	return ctx.state, true
}

func (ctx *Context) mustGetClosestStateNoLock() *GlobalState {
	if ctx.state != nil {
		return ctx.state
	}

	if ctx.parentCtx != nil {
		return ctx.parentCtx.MustGetClosestState()
	}

	panic(ErrNoAssociatedState)
}

func (ctx *Context) getClosestState() (*GlobalState, bool) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return ctx.getClosestStateNoLock()
}

func (ctx *Context) getClosestStateNoLock() (*GlobalState, bool) {
	if ctx.state != nil {
		return ctx.state, true
	}

	if ctx.parentCtx != nil {
		return ctx.parentCtx.getClosestState()
	}

	return nil, false
}

func (ctx *Context) SetClosestState(state *GlobalState) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	if ctx.state != nil {
		panic(ErrAlreadyAssociatedState)
	}

	ctx.state = state
	for _, limiter := range ctx.limiters {
		limiter.SetStateOnce(state.id)
	}
}

func (ctx *Context) Logger() zerolog.Logger {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return ctx.loggerNoLock()
}

func (ctx *Context) loggerNoLock() zerolog.Logger {
	if ctx.state != nil {
		if ctx.state.OutputFieldsInitialized.Load() {
			return ctx.state.Logger

		} //ELSE:
		//originally we waited up to 10 ms for the output fields to be initialized,
		//but that was causing 10ms pauses in Module.Preinit. The Preinit method has been updated
		//but since it could happen in other places, the waiting code has been removed preventively.
	}

	if ctx.parentCtx != nil {
		closestState, ok := ctx.parentCtx.getClosestState()
		if ok {
			return closestState.Logger
		}
	}

	return nopLogger
}

func (ctx *Context) NewChildLoggerForInternalSource(src string) zerolog.Logger {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	state, ok := ctx.getClosestStateNoLock()
	if !ok || !state.OutputFieldsInitialized.Load() {
		return nopLogger
	}

	return childLoggerForInternalSource(state.Logger, src, state.LogLevels)
}

func (ctx *Context) LoggerPrint(args ...any) {
	logger := ctx.Logger()
	logger.Print(args...)
}

func (ctx *Context) LoggerPrintln(args ...any) {
	logger := ctx.Logger()
	logger.Println(args...)
}

func (ctx *Context) DebugLogEvent() *zerolog.Event {
	logger := ctx.Logger()
	return logger.Debug()
}

func (ctx *Context) InfoLogEvent() *zerolog.Event {
	logger := ctx.Logger()
	return logger.Info()
}

func (ctx *Context) WarnLogEvent() *zerolog.Event {
	logger := ctx.Logger()
	return logger.Warn()
}

func (ctx *Context) ErrLogEvent(err error) *zerolog.Event {
	logger := ctx.Logger()
	return logger.Err(err)
}

func (ctx *Context) HasCurrentTx() bool {
	ctx.currentTxLock.Lock()
	defer ctx.currentTxLock.Unlock()
	ctx.assertNotDone()

	return ctx.currentTx != nil
}

func (ctx *Context) GetTx() *Transaction {
	ctx.currentTxLock.Lock()
	defer ctx.currentTxLock.Unlock()
	ctx.assertNotDone()

	if ctx.currentTx != nil {
		return ctx.currentTx
	}
	if ctx.parentCtx != nil {
		return ctx.parentCtx.GetTx()
	}
	return nil
}

// setTx is called by the associated transaction when it starts or finishes.
func (ctx *Context) setTx(tx *Transaction) {
	ctx.currentTxLock.Lock()
	defer ctx.currentTxLock.Unlock()

	ctx.currentTx = tx
}

func (ctx *Context) Now() DateTime {
	return DateTime(time.Now())
}

func (ctx *Context) GetTempDir() Path {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	ctx.assertNotDone()

	if ctx.tempDir == "" {
		ctx.tempDir = CreateTempdir("ctx", ctx.fs)
	}

	return ctx.tempDir
}

func (ctx *Context) GetFileSystem() afs.Filesystem {
	return ctx.fs
}

func (ctx *Context) InitialWorkingDirectory() Path {
	return ctx.initialWorkingDirectory
}

func (ctx *Context) SetWaitConfirmPrompt(fn WaitConfirmPrompt) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	ctx.waitConfirmPrompt = fn
}

func (ctx *Context) GetWaitConfirmPrompt() WaitConfirmPrompt {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	return ctx.waitConfirmPrompt
}

// HasPermission checks if the passed permission is present in the Context.
// The passed permission is first checked against forbidden permissions: if it is included in one of them, false is returned.
func (ctx *Context) HasPermission(perm Permission) bool {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	return ctx.hasPermission(perm)
}

func (ctx *Context) hasPermission(perm Permission) bool {

	for _, forbiddenPerm := range ctx.forbiddenPermissions {
		if forbiddenPerm.Includes(perm) {
			return false
		}
	}

	for _, grantedPerm := range ctx.grantedPermissions {
		if grantedPerm.Includes(perm) {
			return true
		}
	}
	return false
}

// THIS FUNCTION SHOULD NEVER BE USED apart from the symbolic package
func (ctx *Context) HasPermissionUntyped(perm any) bool {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return ctx.hasPermission(perm.(Permission))
}

// THIS FUNCTION SHOULD NEVER BE USED apart from the symbolic package
func (ctx *Context) HasAPermissionWithKindAndType(kind permbase.PermissionKind, typename permbase.InternalPermissionTypename) bool {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	for _, grantedPerm := range ctx.grantedPermissions {
		if grantedPerm.Kind() == kind && grantedPerm.InternalPermTypename() == typename {
			return true
		}
	}
	return false
}

// CheckHasPermission checks if the passed permission is present in the Context, if the permission is not present
// a NotAllowedError is returned.
func (ctx *Context) CheckHasPermission(perm Permission) error {
	if ctx.done.Load() {
		return ctx.makeDoneContextError()
	}

	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	if !ctx.hasPermission(perm) {
		return NewNotAllowedError(perm)
	}

	return nil
}

type BoundChildContextOptions struct {
	AdditionalParentContext context.Context
	Filesystem              afs.Filesystem
	Limits                  []Limit //if nil, defaults to the parent's limits
}

// BoundChild creates a child of the context that also inherits callbacks, named patterns, and protocol clients.
func (ctx *Context) BoundChild() *Context {
	return ctx.boundChild(BoundChildContextOptions{})
}

func (ctx *Context) BoundChildWithOptions(opts BoundChildContextOptions) *Context {
	return ctx.boundChild(opts)
}

func (ctx *Context) boundChild(opts BoundChildContextOptions) *Context {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	ctx.assertNotDone()

	limits := opts.Limits
	if limits == nil {
		limits = ctx.limits
	}

	child := NewContext(ContextConfig{
		Permissions:             ctx.grantedPermissions,
		ForbiddenPermissions:    ctx.forbiddenPermissions,
		Limits:                  limits,
		ParentContext:           ctx,
		AdditionalParentContext: opts.AdditionalParentContext,

		Filesystem: opts.Filesystem,
	})

	child.namedPatterns = ctx.namedPatterns
	child.patternNamespaces = ctx.patternNamespaces
	child.hostProtocolClients = ctx.hostProtocolClients
	child.urlProtocolClients = ctx.urlProtocolClients
	child.typeExtensions = slices.Clone(child.typeExtensions)

	return child
}

// New creates a new context with the same permissions, limits, host data, patterns, aliases & protocol clients,
// if the context has no parent the token counts are copied, the new context does not "share" data with the older context.
func (ctx *Context) New() *Context {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	var limitTokens map[string]int64

	if ctx.parentCtx == nil {
		limitTokens = make(map[string]int64, len(ctx.limiters))

		for limitName, limiter := range ctx.limiters {
			limitTokens[limitName] = limiter.Available()
		}
	}

	clone := NewContext(ContextConfig{
		Permissions:          ctx.grantedPermissions,
		ForbiddenPermissions: ctx.forbiddenPermissions,
		Limits:               ctx.limits,
		HostDefinitions:      ctx.hostDefinitions,
		TypeExtensions:       ctx.typeExtensions,
		ParentContext:        ctx.parentCtx,
		Filesystem:           ctx.fs,

		WaitConfirmPrompt: ctx.waitConfirmPrompt,
	})

	clone.namedPatterns = maps.Clone(ctx.namedPatterns)
	clone.patternNamespaces = maps.Clone(ctx.patternNamespaces)
	//TODO: clone clients ?
	clone.hostProtocolClients = maps.Clone(ctx.hostProtocolClients)
	clone.urlProtocolClients = maps.Clone(ctx.urlProtocolClients)
	clone.userData = maps.Clone(ctx.userData)

	return clone
}

// DropPermissions removes all passed permissions from the context.
func (ctx *Context) DropPermissions(droppedPermissions []Permission) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	var grantedPerms []Permission

top:
	for _, perm := range ctx.grantedPermissions {
		for _, removedPerm := range droppedPermissions {
			//if the granted permission is dropped we check the next granted permission.
			if removedPerm.Includes(perm) {
				continue top
			}
		}

		grantedPerms = append(grantedPerms, perm)
	}

	ctx.grantedPermissions = grantedPerms
	ctx.forbiddenPermissions = append(ctx.forbiddenPermissions, droppedPermissions...)
}

// Take takes an amount of tokens from the bucket associated with a limit.
// The token count is scaled so the passed count is not the took amount.
func (ctx *Context) Take(limitName string, count int64) error {
	if ctx.done.Load() {
		return ctx.makeDoneContextError()
	}

	limiter, ok := ctx.limiters[limitName]
	if !ok {
		//we panic to make sure the execution of the module stops.
		panic(fmt.Errorf("%w: %s", ErrLimitNotPresentInContext, limitName))
	}

	return ctx.DoIO(func() error {
		limiter.Take(count)
		return nil
	})
}

// GiveBack gives backs an amount of tokens from the bucket associated with a limit.
// The token count is scaled so the passed count is not the given back amount.
func (ctx *Context) GiveBack(limitName string, count int64) error {
	if ctx.done.Load() {
		return ctx.makeDoneContextError()
	}

	scaledCount := TOKEN_BUCKET_CAPACITY_SCALE * count

	limiter, ok := ctx.limiters[limitName]
	if !ok {
		//we panic to make sure the execution of the module stops.
		panic(fmt.Errorf("%w: %s", ErrLimitNotPresentInContext, limitName))
	}

	limiter.GiveBack(scaledCount)
	return nil
}

func (ctx *Context) PauseTokenDepletion(limitName string) error {
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.PauseDepletion()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) PauseCPUTimeDepletionIfNotPaused() error {
	limitName := EXECUTION_CPU_TIME_LIMIT_NAME
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.PauseDepletionIfNotPaused()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) DefinitelyStopTokenDepletion(limitName string) error {
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.DefinitelyStopDepletion()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) ResumeDepletion(limitName string) error {
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.ResumeDepletion()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) PauseCPUTimeDepletion() error {
	return ctx.PauseTokenDepletion(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) ResumeCPUTimeDepletion() error {
	return ctx.ResumeDepletion(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) DefinitelyStopCPUTimeDepletion() error {
	return ctx.DefinitelyStopTokenDepletion(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) DoIO(fn func() error) error {
	ctx.PauseCPUTimeDepletion()
	defer ctx.ResumeCPUTimeDepletion()

	//do not recover from panics on purpose
	return fn()
}

func DoIO[T any](ctx *Context, fn func() T) T {
	ctx.PauseCPUTimeDepletion()
	defer ctx.ResumeCPUTimeDepletion()

	//do not recover from panics on purpose
	return fn()
}

func DoIO2[T any](ctx *Context, fn func() (T, error)) (T, error) {
	ctx.PauseCPUTimeDepletion()
	defer ctx.ResumeCPUTimeDepletion()

	//do not recover from panics on purpose
	return fn()
}

func (ctx *Context) Sleep(duration time.Duration) {
	ctx.PauseCPUTimeDepletion()
	defer ctx.ResumeCPUTimeDepletion()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		// add pause ?
	}
}

// GetByteRate returns the value (rate) of a byte rate limit.
func (ctx *Context) GetByteRate(name string) (ByteRate, error) {
	if ctx.done.Load() {
		return -1, ctx.makeDoneContextError()
	}

	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	limiter, ok := ctx.limiters[name]
	if !ok {
		return -1, fmt.Errorf("context: cannot get rate '%s': not present", name)
	}
	if limiter.limit.Kind != ByteRateLimit {
		return -1, fmt.Errorf("context: '%s' is not a rate", name)
	}
	return ByteRate(limiter.limit.Value), nil
}

// GetTotal returns the value of a limit of kind total.
func (ctx *Context) GetTotal(name string) (int64, error) {
	if ctx.done.Load() {
		return -1, ctx.makeDoneContextError()
	}

	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	limiter, ok := ctx.limiters[name]
	if !ok {
		return -1, fmt.Errorf("context: cannot get total '%s': not present", name)
	}

	return limiter.Total()
}

func (ctx *Context) GetGrantedPermissions() []Permission {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return slices.Clone(ctx.grantedPermissions)
}

func (ctx *Context) GetForbiddenPermissions() []Permission {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return slices.Clone(ctx.forbiddenPermissions)
}

func (ctx *Context) Limits() []Limit {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return slices.Clone(ctx.limits)
}

type LockedContextData struct {
	NamedPatterns     map[string]Pattern
	PatternNamespaces map[string]*PatternNamespace
	HostAliases       map[string]Host
}

// Update locks the context (Lock) and calls fn. fn is allowed to modify the context data.
func (ctx *Context) Update(fn func(ctxData LockedContextData) error) error {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return fn(LockedContextData{
		NamedPatterns:     ctx.namedPatterns,
		PatternNamespaces: ctx.patternNamespaces,
	})
}

// ResolveNamedPattern returns the pattern associated with the passed name, if the pattern does not exist nil is returned.
func (ctx *Context) ResolveNamedPattern(name string) Pattern {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	pattern, ok := ctx.namedPatterns[name]
	if !ok {
		return nil
	}
	return pattern
}

func (ctx *Context) GetNamedPatterns() map[string]Pattern {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return maps.Clone(ctx.namedPatterns)
}

func (ctx *Context) ForEachNamedPattern(fn func(name string, pattern Pattern) error) error {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	for k, v := range ctx.namedPatterns {
		err := fn(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddNamedPattern associates a Pattern with the passed pattern name, if the pattern is already defined the function will panic.
func (ctx *Context) AddNamedPattern(name string, pattern Pattern) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.namedPatterns[name]
	//_, isDynamic := patt.(*DynamicStringPatternElement)

	if ok {
		panic(fmt.Errorf("%w: %s", ErrNotUniquePatternDefinition, name))
	}
	ctx.namedPatterns[name] = pattern
}

// ResolvePatternNamespace returns the pattern namespace associated with the passed name, if the namespace does not exist nil is returned.
func (ctx *Context) ResolvePatternNamespace(name string) *PatternNamespace {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	namespace, ok := ctx.patternNamespaces[name]
	if !ok {
		return nil
	}
	return namespace
}

// AddPatternNamespace associates a *PatternNamespace with the passed pattern name, if the pattern is already defined the function will panic.
func (ctx *Context) AddPatternNamespace(name string, namespace *PatternNamespace) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.patternNamespaces[name]

	if ok {
		panic(fmt.Errorf("%w: %s", ErrNotUniquePatternNamespaceDefinition, name))
	}
	ctx.patternNamespaces[name] = namespace
}

func (ctx *Context) GetPatternNamespaces() map[string]*PatternNamespace {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return maps.Clone(ctx.patternNamespaces)
}

func (ctx *Context) ForEachPatternNamespace(fn func(name string, namespace *PatternNamespace) error) error {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	for k, v := range ctx.patternNamespaces {
		err := fn(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ctx *Context) SetProtocolClientForURL(u URL, client ProtocolClient) error {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	if _, alreadyExist := ctx.urlProtocolClients[u]; alreadyExist {
		return ErrNotUniqueProtocolClient
	}

	ctx.urlProtocolClients[u] = client
	return nil
}

func (ctx *Context) SetProtocolClientForHost(h Host, client ProtocolClient) error {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	if _, alreadyExist := ctx.hostProtocolClients[h]; alreadyExist {
		return ErrNotUniqueProtocolClient
	}

	ctx.hostProtocolClients[h] = client
	return nil
}

func (ctx *Context) GetProtolClient(u URL) (ProtocolClient, error) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	// if name == "default" {
	// 	return &HttpClient{
	// 		Config: DEFAULT_HTTP_PROFILE_CONFIG,
	// 	}, nil
	// }

	client, ok := ctx.urlProtocolClients[u]

	if !ok {
		client, ok = ctx.hostProtocolClients[u.Host()]
		if !ok {
			return nil, fmt.Errorf("protocol client for URL '%s' / Host '%s' / scheme '%s' does not exist", u, u.Host(), u.Scheme())
		}
	}

	return client, nil
}

func (ctx *Context) GetHostDefinition(h Host) Value {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	v, ok := ctx.hostDefinitions[h]
	if !ok {
		return Nil
	}

	return v
}

func (ctx *Context) GetHostByDefinition(r ResourceName) (Host, bool) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	for host, data := range ctx.hostDefinitions {
		if data == r {
			return host, true
		}
	}
	return "", false
}

func (ctx *Context) GetAllHostDefinitions() map[Host]Value {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return maps.Clone(ctx.hostDefinitions)
}

// AddHostDefinition adds a host definition, redefining a host causes the function to panic with ErrNotUniqueHostDefinitionDefinition.
func (ctx *Context) AddHostDefinition(h Host, data ResourceName) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.hostDefinitions[h]

	if ok {
		panic(fmt.Errorf("%w: %s", ErrNotUniqueHostDefinitionDefinition, h))
	}
	ctx.hostDefinitions[h] = data
}

// ResolveUserData returns the user data associated with the passed identifier, if the data does not exist nil is returned.
func (ctx *Context) ResolveUserData(path Path) Value {
	ctx.lock.RLock()
	unlock := true
	defer func() {
		if unlock {
			ctx.lock.RUnlock()
		}
	}()

	data, ok := ctx.userData[path]
	if !ok {
		if ctx.parentCtx != nil {
			unlock = false
			ctx.lock.RUnlock()
			return ctx.parentCtx.ResolveUserData(path)
		}
		return nil
	}
	return data
}

// PutUserData associates $value with the passed name, if the entry is already defined the function will panic.
// $value must be sharable or clonable.
func (ctx *Context) PutUserData(path Path, value Value) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.userData[path]
	if ok {
		panic(fmt.Errorf("%w: %s", ErrDoubleUserDataDefinition, path))
	}

	shared, err := ShareOrClone(value, ctx.mustGetClosestStateNoLock())
	if err != nil {
		if errors.Is(err, ErrValueNotSharableNorClonable) {
			panic(ErrNotSharableNorClonableUserDataValue)
		} else {
			panic(err)
		}
	}
	ctx.userData[path] = shared
}

func (ctx *Context) GetTypeExtension(id string) *TypeExtension {
	for _, ext := range ctx.typeExtensions {
		if ext.Id() == id {
			return ext
		}
	}
	return nil
}

func (ctx *Context) GetAllTypeExtensions() []*TypeExtension {
	return slices.Clone(ctx.typeExtensions)
}

func (ctx *Context) AddTypeExtension(extension *TypeExtension) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	for _, ext := range ctx.typeExtensions {
		if extension.Id() == ext.Id() {
			propertyNames := utils.MapSlice(extension.propertyExpressions, func(e propertyExpression) string {
				return e.name
			})

			propertyList := strings.Join(propertyNames, ", ")
			panic(fmt.Errorf("%w, the properties of the extension are: %s", ErrTypeExtensionAlreadyRegistered, propertyList))
		}
	}

	ctx.typeExtensions = append(ctx.typeExtensions, extension)
}

func (ctx *Context) IsValueVisible(v Value) bool {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return true
}

func (ctx *Context) PromoteToLongLived() {
	if ctx.longLived.CompareAndSwap(false, true) {
		//TODO:
	}
}

func (ctx *Context) IsLongLived() bool {
	return ctx.longLived.Load()
}

// CancelGracefully calls the graceful teardown tasks one by one synchronously, then the context is truly cancelled.
// CancelGracefully does not wait for the ungraceful teardown that follows the graceful one to complete.
// TODO: add cancellation cause
func (ctx *Context) CancelGracefully() {
	ignore := ctx.gracefullyTearDown()
	if ignore {
		return
	}

	if ctx.done.CompareAndSwap(false, true) {
		ctx.lock.Lock()
		defer ctx.lock.Unlock()
		ctx.cancel() // TODO: prevent deadlock
	}
}

// CancelUngracefully directly cancels the go context, CancelGracefully should always be called instead.
// CancelUngracefully does not wait for the ungraceful teardown to complete.
func (ctx *Context) CancelUngracefully() {
	if ctx.done.CompareAndSwap(false, true) {
		ctx.lock.Lock()
		defer ctx.lock.Unlock()

		if ctx.GracefulTearDownStatus() == GracefullyTearingDown {
			ctx.gracefulTearDownStatus.Store(int64(GracefullyTearedDownWithCancellation))
		}
		ctx.cancel() // TODO: prevent deadlock
	}
}

func (ctx *Context) CancelIfShortLived() {
	if !ctx.longLived.Load() {
		ctx.CancelGracefully()
	}
}

func (ctx *Context) gracefullyTearDown() (ignore bool) {
	if ctx.done.Load() {
		ignore = true
		return
	}

	if !ctx.gracefulTearDownStatus.CompareAndSwap(int64(NeverStartedGracefulTeardown), int64(GracefullyTearingDown)) {
		ignore = true
		return
	}

	ctx.gracefulTearDownCallLocation.Store(string(debug.Stack()))
	errorDuringTeardown := false
	cancellationDuringTeardown := false

	defer func() {
		status := GracefullyTearedDown
		if cancellationDuringTeardown {
			status = GracefullyTearedDownWithCancellation
		} else if errorDuringTeardown {
			status = GracefullyTearedDownWithErrors
		}
		defer ctx.gracefulTearDownStatus.Store(int64(status))
	}()

	logger := ctx.Logger()

	defer func() {
		ctx.onGracefulTearDownTasks = nil
	}()

	for _, taskFn := range ctx.onGracefulTearDownTasks {
		if utils.IsContextDone(ctx) {
			cancellationDuringTeardown = true
			break
		}

		func() {
			defer func() {
				if e := recover(); e != nil {
					errorDuringTeardown = true
					defer utils.Recover()

					err := fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), string(debug.Stack()))
					logger.Err(err).Msg("error while calling a context teardown task")
				}
			}()

			err := taskFn(ctx)
			if err != nil {
				logger.Err(err).Msg("error returned by a context teardown task")
				errorDuringTeardown = true
			}
		}()
	}

	return
}

func (ctx *Context) ToSymbolicValue() (*symbolic.Context, error) {
	symbolicCtx := symbolic.NewSymbolicContext(ctx, ctx, nil)

	for k, v := range ctx.namedPatterns {
		symbolicVal, err := ToSymbolicValue(ctx, v, false)
		if err != nil {
			return nil, fmt.Errorf("cannot convert named pattern %s: %s", k, err)
		}

		symbolicCtx.AddNamedPattern(k, symbolicVal.(symbolic.Pattern), false)
	}

	for k, v := range ctx.patternNamespaces {
		symbolicVal, err := ToSymbolicValue(ctx, v, false)
		if err != nil {
			return nil, fmt.Errorf("cannot convert '%s' pattern namespace: %s", k, err)
		}

		symbolicCtx.AddPatternNamespace(k, symbolicVal.(*symbolic.PatternNamespace), false)
	}

	return symbolicCtx, nil
}

type IWithSecondaryContext interface {
	//context should not be nil
	WithSecondaryContext(*Context) any

	WithoutSecondaryContext() any
}

func WithSecondaryContextIfPossible[T any](ctx *Context, arg T) T {
	if itf, ok := any(arg).(IWithSecondaryContext); ok {
		return itf.WithSecondaryContext(ctx).(T)
	}
	return arg
}

func WithoutSecondaryContextIfPossible[T any](arg T) T {
	if itf, ok := any(arg).(IWithSecondaryContext); ok {
		return itf.WithoutSecondaryContext().(T)
	}
	return arg
}
