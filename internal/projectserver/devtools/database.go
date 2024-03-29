package devtools

import (
	"errors"
	"sync"

	"github.com/inoxlang/inox/internal/core"
)

var (
	errDbNotAvailable = errors.New("database not available")
)

// A dbProxy allows a tooling program to interact with an Inox dbProxy.
type dbProxy struct {
	dbName   string
	instance *Instance
	lock     sync.Mutex

	current *core.DatabaseIL
}

func newDBProxy(name string, session *Instance) *dbProxy {
	return &dbProxy{
		dbName:   name,
		instance: session,
	}
}

type databaseOpeningConfig struct {
	open   core.OpenDBFn
	config core.DbOpenConfiguration
}

func (p *dbProxy) getSchema(ctx *core.Context) (*core.ObjectPattern, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	lockSession := true

	db, err := p.dbNoLock(lockSession)
	if err != nil {
		return core.NewInexactObjectPattern([]core.ObjectPatternEntry{}), err
	}

	return db.Schema(), nil
}

func (p *dbProxy) dbNoLock(lockSession bool) (*core.DatabaseIL, error) {
	if p.current != nil {
		return p.current, nil
	}

	if lockSession {
		p.instance.lock.Lock()
		defer p.instance.lock.Unlock()
	}

	db, ok := p.instance.runningProgramDatabases[p.dbName]
	if ok {
		p.current = db
		return db, nil
	}

	config, ok := p.instance.databaseOpeningConfigurations[p.dbName]
	if ok {
		dbLower, err := config.open(p.instance.context, config.config)
		if err != nil {
			return nil, err
		}
		db, err := core.WrapDatabase(p.instance.context, core.DatabaseWrappingArgs{
			Name:  p.dbName,
			Inner: dbLower,
		})
		if err != nil {
			return nil, err
		}

		p.current = nil
		return db, nil
	}

	return nil, errDbNotAvailable
}
