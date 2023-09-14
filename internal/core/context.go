package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	EXECUTION_TOTAL_LIMIT_NAME = "execution/total-time"

	// Note:
	// This limit represents a pseudo CPU time because it's not possible to accurately detect when
	// the goroutine executing a module is waiting for IO.
	//
	// Implementation note:
	// CPU time decrementation should not be paused during lockings that are both shorts & often successful on the first try
	// because it would introduce overhead. Pausing the decrementation involves an atomic write.
	EXECUTION_CPU_TIME_LIMIT_NAME = "execution/cpu-time"
)

var (
	ErrNonExistingNamedPattern                 = errors.New("non existing named pattern")
	ErrNotUniqueAliasDefinition                = errors.New("cannot register a host alias more than once")
	ErrNotUniquePatternDefinition              = errors.New("cannot register a pattern more than once")
	ErrNotUniquePatternNamespaceDefinition     = errors.New("cannot register a pattern namespace more than once")
	ErrNotUniqueHostResolutionDefinition       = errors.New("cannot set host resolution data more than once")
	ErrNotUniqueProtocolClient                 = errors.New("client already defined")
	ErrCannotProvideLimitTokensForChildContext = errors.New("limit tokens cannot be set in new context's config if it is a child")
	ErrNoAssociatedState                       = errors.New("context has no associated state")
	ErrAlreadyAssociatedState                  = errors.New("context already has an associated state")
	ErrNotSharableUserDataValue                = errors.New("attempt to set a user data entry with a non sharable value ")
	ErrDoubleUserDataDefinition                = errors.New("cannot define a user data entry more than once")
)

type Context struct {
	context.Context

	kind      ContextKind
	lock      sync.RWMutex
	parentCtx *Context

	// associated state, it is set at the creation of the associated state.
	state *GlobalState
	fs    afs.Filesystem

	currentTx *Transaction

	longLived atomic.Bool
	done      atomic.Bool
	cancel    context.CancelFunc

	//permissions & limits
	grantedPermissions   []Permission
	forbiddenPermissions []Permission
	limits               []Limit
	limiters             map[string]*limiter //the map is not changed after context creation

	//values
	hostAliases         map[string]Host
	namedPatterns       map[string]Pattern
	patternNamespaces   map[string]*PatternNamespace
	urlProtocolClients  map[URL]ProtocolClient
	hostProtocolClients map[Host]ProtocolClient
	hostResolutionData  map[Host]Value
	userData            map[Identifier]Value

	executionStartTime time.Time

	tempDir           Path //directory for storing temporary files, defaults to a random directory in /tmp
	waitConfirmPrompt WaitConfirmPrompt
}

type ContextKind int

const (
	DefaultContext ContextKind = iota
	TestingContext
)

type ContextConfig struct {
	Kind                 ContextKind
	Permissions          []Permission
	ForbiddenPermissions []Permission

	//if (cpu time limit is not present) AND (parent context has it) then the limit is inherited.
	//The decrementation of total limit's tokens for the created context starts when the associated state is set.
	Limits []Limit

	HostResolutions  map[Host]Value
	OwnedDatabases   []DatabaseConfig
	ParentContext    *Context
	LimitTokens      map[string]int64
	Filesystem       afs.Filesystem
	CreateFilesystem func(ctx *Context) (afs.Filesystem, error)

	WaitConfirmPrompt WaitConfirmPrompt
}

type WaitConfirmPrompt func(msg string, accepted []string) (bool, error)

func (c ContextConfig) HasParentRequiredPermissions() (firstErr error, ok bool) {
	if c.ParentContext == nil {
		return nil, true
	}

top:
	for _, perm := range c.Permissions {

		dbPerm, ok := perm.(DatabasePermission)
		if ok {
			for _, dbConfig := range c.OwnedDatabases {
				if dbConfig.IsPermissionForThisDB(dbPerm) {
					continue top
				}
			}
		}

		if err := c.ParentContext.CheckHasPermission(perm); err != nil {
			return fmt.Errorf("parent of context should at least have permissions of its child: %w", err), false
		}
	}
	return nil, true
}

func NewContexWithEmptyState(config ContextConfig, out io.Writer) *Context {
	ctx := NewContext(config)
	state := NewGlobalState(ctx)

	if out == nil {
		out = io.Discard
	}
	state.Out = out
	state.Logger = zerolog.New(out)
	return ctx
}

// NewContext creates a new context, if a parent context is provided the embedded context.Context will be
// context.WithCancel(parentContext), otherwise it will be context.WithCancel(context.Background()).
func NewContext(config ContextConfig) *Context {

	var (
		limiters   map[string]*limiter
		stdlibCtx  context.Context
		cancel     context.CancelFunc
		filesystem afs.Filesystem = config.Filesystem
		ctx                       = &Context{} //the context is initialized later in the function but we need the address
	)

	if config.CreateFilesystem != nil {
		if filesystem != nil {
			panic(fmt.Errorf("invalid arguments: both .CreateFilesystem & .Filesystem provided"))
		}

		fs, err := config.CreateFilesystem(ctx)
		if err != nil {
			panic(err)
		}
		filesystem = fs
	}

	//create limiters
	limiters = make(map[string]*limiter)
	var parentContextLimiters map[string]*limiter

	if config.ParentContext != nil {
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
		case ByteRateLimit, SimpleRateLimit:
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
				decrementFn:                  l.DecrementFn,
				cancelContextOnNegativeCount: l.Kind == TotalLimit,
			}),
		}
	}

	if config.ParentContext == nil {
		stdlibCtx, cancel = context.WithCancel(context.Background())
	} else {
		//if a parent context is passed we check that the parent has all the required permissions
		if err, ok := config.HasParentRequiredPermissions(); !ok {
			panic(fmt.Errorf("failed to create context, parent of context should at least have permissions of its child: %w", err))
		}

		if config.LimitTokens != nil {
			panic(ErrCannotProvideLimitTokensForChildContext)
		}

		//inherit limits from parent
		for _, parentLimiter := range config.ParentContext.limiters {
			limitName := parentLimiter.limit.Name
			if _, ok := limiters[limitName]; !ok {
				limiters[limitName] = parentLimiter.Child()
			}
		}

		stdlibCtx, cancel = context.WithCancel(config.ParentContext)

		if filesystem == nil {
			filesystem = config.ParentContext.fs
		}
	}

	hostResolutions := maps.Clone(config.HostResolutions)

	*ctx = Context{
		kind:                 config.Kind,
		Context:              stdlibCtx,
		cancel:               cancel,
		parentCtx:            config.ParentContext,
		fs:                   WithSecondaryContextIfPossible(ctx, filesystem),
		executionStartTime:   time.Now(),
		grantedPermissions:   utils.CopySlice(config.Permissions),
		forbiddenPermissions: utils.CopySlice(config.ForbiddenPermissions),
		limits:               utils.CopySlice(config.Limits),
		limiters:             limiters,
		hostAliases:          map[string]Host{},
		namedPatterns:        map[string]Pattern{},
		patternNamespaces:    map[string]*PatternNamespace{},
		urlProtocolClients:   map[URL]ProtocolClient{},
		hostProtocolClients:  map[Host]ProtocolClient{},
		hostResolutionData:   hostResolutions,
		userData:             map[Identifier]Value{},

		waitConfirmPrompt: config.WaitConfirmPrompt,
	}

	for _, limiter := range limiters {
		limiter.SetContextIfNotChild(ctx)
	}

	//cleanup
	go func() {
		<-ctx.Done()

		ctx.lock.Lock()
		defer ctx.lock.Unlock()

		//ctx.finished = true

		for _, limiter := range ctx.limiters {
			limiter.Destroy()
		}

		//release acquired resources
		//TODO

		//rollback transaction (the rollback will be ignored if the transaction is finished)
		if ctx.currentTx != nil {
			ctx.currentTx.Rollback(ctx)
		}
	}()

	return ctx
}

func (ctx *Context) makeDoneContextError() error {
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

func (ctx *Context) GetClosestState() *GlobalState {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	return ctx.getClosestStateNoLock()
}

func (ctx *Context) getClosestStateNoLock() *GlobalState {
	ctx.assertNotDone()

	if ctx.state != nil {
		return ctx.state
	}

	if ctx.parentCtx != nil {
		return ctx.parentCtx.GetClosestState()
	}

	panic(ErrNoAssociatedState)
}

func (ctx *Context) Logger() *zerolog.Logger {
	return &ctx.GetClosestState().Logger
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

func (ctx *Context) HasCurrentTx() bool {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	ctx.assertNotDone()

	return ctx.currentTx != nil
}

func (ctx *Context) GetTx() *Transaction {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
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
	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	ctx.currentTx = tx
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
func (ctx *Context) HasAPermissionWithKindAndType(kind permkind.PermissionKind, typename permkind.InternalPermissionTypename) bool {
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
	Filesystem afs.Filesystem
}

// BoundChild creates a child of the context that also inherits callbacks, named patterns, host aliases and protocol clients.
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

	child := NewContext(ContextConfig{
		Permissions:          ctx.grantedPermissions,
		ForbiddenPermissions: ctx.forbiddenPermissions,
		Limits:               ctx.limits,
		HostResolutions:      ctx.hostResolutionData,
		ParentContext:        ctx,

		Filesystem: opts.Filesystem,
	})

	child.namedPatterns = ctx.namedPatterns
	child.patternNamespaces = ctx.patternNamespaces
	child.hostAliases = ctx.hostAliases
	child.hostProtocolClients = ctx.hostProtocolClients
	child.urlProtocolClients = ctx.urlProtocolClients
	child.userData = ctx.userData

	return child
}

// ChildWithout creates a new Context with the permissions passed as argument removed.
// The limiters are shared between the two contexts.
func (ctx *Context) ChildWithout(removedPerms []Permission) (*Context, error) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
	ctx.assertNotDone()

	var perms []Permission
	var forbiddenPerms []Permission

top:
	for _, perm := range ctx.grantedPermissions {
		for _, removedPerm := range removedPerms {
			if removedPerm.Includes(perm) {
				continue top
			}
		}

		perms = append(perms, perm)
	}

	return NewContext(ContextConfig{
		Permissions:          perms,
		ForbiddenPermissions: forbiddenPerms,
		ParentContext:        ctx,
	}), nil
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
		HostResolutions:      ctx.hostResolutionData,
		ParentContext:        ctx.parentCtx,
		Filesystem:           ctx.fs,

		WaitConfirmPrompt: ctx.waitConfirmPrompt,
	})

	clone.namedPatterns = maps.Clone(ctx.namedPatterns)
	clone.patternNamespaces = maps.Clone(ctx.patternNamespaces)
	clone.hostAliases = maps.Clone(ctx.hostAliases)
	//TODO: clone clients ?
	clone.hostProtocolClients = maps.Clone(ctx.hostProtocolClients)
	clone.urlProtocolClients = maps.Clone(ctx.urlProtocolClients)
	clone.userData = maps.Clone(ctx.userData)

	return clone
}

// NewWith creates a new independent Context with the context's permissions, limits + passed permissions.
func (ctx *Context) NewWith(additionalPerms []Permission) (*Context, error) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	var perms []Permission = make([]Permission, len(ctx.grantedPermissions))
	copy(perms, ctx.grantedPermissions)

top:
	for _, additonalPerm := range additionalPerms {
		for _, perm := range perms {
			if perm.Includes(additonalPerm) {
				continue top
			}
		}

		perms = append(perms, additonalPerm)
	}

	newCtx := NewContext(ContextConfig{
		Permissions:          perms,
		ForbiddenPermissions: ctx.forbiddenPermissions,
		Limits:               ctx.limits,
		Filesystem:           ctx.fs,

		WaitConfirmPrompt: ctx.waitConfirmPrompt,
	})
	return newCtx, nil
}

// DropPermissions removes all passed permissions from the context.
func (ctx *Context) DropPermissions(droppedPermissions []Permission) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()
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

	ctx.lock.Lock()
	defer ctx.lock.Unlock()

	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.Take(count)
	}

	return nil
}

// GiveBack gives backs an amount of tokens from the bucket associated with a limit.
// The token count is scaled so the passed count is not the given back amount.
func (ctx *Context) GiveBack(limitName string, count int64) error {
	if ctx.done.Load() {
		return ctx.makeDoneContextError()
	}

	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	scaledCount := TOKEN_BUCKET_CAPACITY_SCALE * count

	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.GiveBack(scaledCount)
	}
	return nil
}

func (ctx *Context) PauseDecrementation(limitName string) error {
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.PauseDecrementation()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) PauseCPUDecrementationIfNotPaused() error {
	limitName := EXECUTION_CPU_TIME_LIMIT_NAME
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.PauseDecrementationIfNotPaused()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) ResumeDecrementation(limitName string) error {
	limiter, ok := ctx.limiters[limitName]
	if ok {
		limiter.ResumeDecrementation()
		return nil
	}
	return fmt.Errorf("context: non existing limit '%s'", limitName)
}

func (ctx *Context) PauseCPUTimeDecrementation() error {
	return ctx.PauseDecrementation(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) TryPauseCPUTimeDecrementation() error {
	return ctx.PauseDecrementation(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) ResumeCPUTimeDecrementation() error {
	return ctx.ResumeDecrementation(EXECUTION_CPU_TIME_LIMIT_NAME)
}

func (ctx *Context) DoIO(fn func() error) error {
	ctx.PauseCPUTimeDecrementation()
	defer ctx.ResumeCPUTimeDecrementation()

	return fn()
}

func DoIO[T any](ctx *Context, fn func() T) T {
	ctx.PauseCPUTimeDecrementation()
	defer ctx.ResumeCPUTimeDecrementation()

	return fn()
}

func DoIO2[T any](ctx *Context, fn func() (T, error)) (T, error) {
	ctx.PauseCPUTimeDecrementation()
	defer ctx.ResumeCPUTimeDecrementation()

	return fn()
}

func (ctx *Context) Sleep(duration time.Duration) {
	ctx.PauseCPUTimeDecrementation()
	defer ctx.ResumeCPUTimeDecrementation()

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

	return utils.CopySlice(ctx.grantedPermissions)
}

func (ctx *Context) GetForbiddenPermissions() []Permission {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return utils.CopySlice(ctx.forbiddenPermissions)
}

// ResolveHostAlias returns the Host associated with the passed alias name, if the alias does not exist nil is returned.
func (ctx *Context) ResolveHostAlias(alias string) Host {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	host, ok := ctx.hostAliases[alias]
	if !ok {
		return ""
	}
	return host
}

// AddHostAlias associates a Host with the passed alias name, if the alias is already defined the function will panic.
func (ctx *Context) AddHostAlias(alias string, host Host) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.hostAliases[alias]
	if ok {
		panic(fmt.Errorf("%w: %s", ErrNotUniqueAliasDefinition, alias))
	}
	ctx.hostAliases[alias] = host
}

func (ctx *Context) GetHostAliases() map[string]Host {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return maps.Clone(ctx.hostAliases)
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

func (ctx *Context) GetHostResolutionData(h Host) Value {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	v, ok := ctx.hostResolutionData[h]
	if !ok {
		return Nil
	}

	return v
}

func (ctx *Context) GetHostFromResolutionData(r ResourceName) (Host, bool) {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	for host, data := range ctx.hostResolutionData {
		if data == r {
			return host, true
		}
	}
	return "", false
}

func (ctx *Context) GetAllHostResolutionData() map[Host]Value {
	ctx.lock.RLock()
	defer ctx.lock.RUnlock()

	return maps.Clone(ctx.hostResolutionData)
}

// AddHostResolutionData associates data to a host in
func (ctx *Context) AddHostResolutionData(h Host, data ResourceName) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.hostResolutionData[h]

	if ok {
		panic(fmt.Errorf("%w: %s", ErrNotUniqueHostResolutionDefinition, h))
	}
	ctx.hostResolutionData[h] = data
}

// ResolveUserData returns the user data associated with the passed identifier, if the data does not exist nil is returned.
func (ctx *Context) ResolveUserData(name Identifier) Value {
	ctx.lock.RLock()
	unlock := true
	defer func() {
		if unlock {
			ctx.lock.RUnlock()
		}
	}()

	data, ok := ctx.userData[name]
	if !ok {
		if ctx.parentCtx != nil {
			unlock = false
			ctx.lock.RUnlock()
			return ctx.parentCtx.ResolveUserData(name)
		}
		return nil
	}
	return data
}

// AddUserData associates a data with the passed name, if the data is already defined the function will panic.
func (ctx *Context) AddUserData(name Identifier, value Value) {
	ctx.lock.Lock()
	defer ctx.lock.Unlock()
	ctx.assertNotDone()

	_, ok := ctx.userData[name]
	if ok {
		panic(fmt.Errorf("%w: %s", ErrDoubleUserDataDefinition, name))
	}

	if ok, expl := IsSharable(value, ctx.getClosestStateNoLock()); !ok {
		panic(fmt.Errorf("%w: %s", ErrNotSharableUserDataValue, expl))
	}
	ctx.userData[name] = value
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

// Cancel cancels the Context.
// TODO: add cancellation cause
func (ctx *Context) Cancel() {
	if ctx.done.CompareAndSwap(false, true) {
		ctx.lock.Lock()
		defer ctx.lock.Unlock()

		if ctx.cancel != nil {
			ctx.cancel() // TODO: prevent deadlock
		}
	}
}

func (ctx *Context) CancelIfShortLived() {
	if !ctx.longLived.Load() {
		ctx.Cancel()
	}
}

func (ctx *Context) ToSymbolicValue() (*symbolic.Context, error) {
	symbolicCtx := symbolic.NewSymbolicContext(ctx, nil)

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

	for k, v := range ctx.hostAliases {
		symbolicVal, err := ToSymbolicValue(ctx, v, false)
		if err != nil {
			return nil, fmt.Errorf("cannot convert host alias %s: %s", k, err)
		}

		symbolicCtx.AddHostAlias(k, symbolicVal.(*symbolic.Host), false)
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
