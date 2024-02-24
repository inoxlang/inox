package project

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/project/access"
	"github.com/inoxlang/inox/internal/sourcecontrol"
)

// A developerCopy represents a copy of the project for a single project member.
// It can be used by a single development session.
type developerCopy struct {
	member        *access.Member
	fsDir         string //location of the developer's working tree on the underlying filesystem.
	gitStorageDir string //location of the developer's git storage on the underlying filesystem.
	lock          sync.Mutex

	//The following fields are all nil if the copy is not open (no dev session started).
	//On session termination they are all set to nil.

	devSessionContext *core.Context
	workingFs         *fs_ns.MetaFilesystem //working tree
	gitStorageFs      *fs_ns.MetaFilesystem
	repository        *sourcecontrol.GitRepository
}

func (p *Project) getOpenDevCopy(devSessionContext *core.Context, fsDir, gitStorageDir string, member *access.Member) (*developerCopy, error) {
	p.developerCopiesLock.Lock()
	copy, ok := p.developerCopies[member.ID()]

	if !ok {
		defer p.developerCopiesLock.Unlock()

		copy = &developerCopy{
			member:        member,
			fsDir:         fsDir,
			gitStorageDir: gitStorageDir,
		}

		err := copy.beginNewSession(devSessionContext, p)
		if err != nil {
			return nil, err
		}

		p.developerCopies[member.ID()] = copy
		return copy, nil
	}

	copy.lock.Lock()
	defer copy.lock.Unlock()

	//We no longer need access to p.developerCopies.
	p.developerCopiesLock.Unlock()

	if copy.devSessionContext == devSessionContext { //Same session
		return copy, nil
	}

	if copy.devSessionContext != nil {
		//Wait for the previous development session to finish.
		select {
		case <-copy.devSessionContext.Done():
		case <-time.After(time.Second):
		}

		if !copy.workingFs.IsClosed() && copy.workingFs.IsClosedOrClosing() {
			//Wait for the filesystem to close.
			time.Sleep(time.Second)
		}

		if !copy.workingFs.IsClosed() {
			if copy.workingFs.IsClosedOrClosing() {
				return nil, fmt.Errorf("filesystem of the previous development session is still closing")
			}
			return nil, fmt.Errorf("a development session is already using the filesystem")
		}
	}

	err := copy.beginNewSession(devSessionContext, p)
	if err != nil {
		return nil, err
	}

	return copy, nil
}

// WorkingFilesystem returns the working filesystem (working tree) of the developer's copy.
// It may not be available because the developer (project member) has not started a session.
func (c *developerCopy) WorkingFilesystem() (*fs_ns.MetaFilesystem, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.workingFs == nil || c.workingFs.IsClosed() || c.workingFs.IsClosedOrClosing() {
		return nil, false
	}
	return c.workingFs, true
}

// WorkingFilesystem returns the Git repository of the developer's copy.
// It may not be available because the developer (project member) has not started a session.
func (c *developerCopy) Repository() (*sourcecontrol.GitRepository, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.repository == nil {
		return nil, false
	}
	return c.repository, true
}

// beginNewSession (re) opens the filesystem and local repository of the developer's copy. The filesystem will be closed
// and c.workingFs, c.devSessionContext will be set to nil after the session finishes.
func (c *developerCopy) beginNewSession(devSessionContext *core.Context, project *Project) error {
	c.workingFs = nil
	c.gitStorageFs = nil
	c.devSessionContext = nil
	c.repository = nil

	workingFS, err := fs_ns.OpenMetaFilesystem(devSessionContext, project.osFilesystem, fs_ns.MetaFilesystemParams{
		Dir:            c.fsDir,
		MaxUsableSpace: project.maxFilesystemSize,
	})

	if err != nil {
		return fmt.Errorf("failed to open the copy (filesystem) of the project %s for member %s: %w", project.Id(), c.member.Name(), err)
	}

	gitStorageFS, err := fs_ns.OpenMetaFilesystem(devSessionContext, project.osFilesystem, fs_ns.MetaFilesystemParams{
		Dir:            c.gitStorageDir,
		MaxUsableSpace: project.maxFilesystemSize,
	})

	if err != nil {
		return fmt.Errorf("failed to open the git storage filesystem of the project %s for member %s: %w", project.Id(), c.member.Name(), err)
	}

	c.workingFs = workingFS
	c.gitStorageFs = gitStorageFS
	c.devSessionContext = devSessionContext

	c.devSessionContext.OnGracefulTearDown(func(ctx *core.Context) error {
		defer gitStorageFS.Close(ctx)

		workingFS.Close(ctx)
		return nil
	})

	c.devSessionContext.OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		//Remove the references to the context and the filesystem to allow garbage collection.
		c.lock.Lock()

		if c.devSessionContext == devSessionContext {
			c.devSessionContext = nil
			c.workingFs = nil
			c.gitStorageFs = nil
			c.repository = nil
		}

		c.lock.Unlock()
		return nil
	})

	return c.openRepositoryNoLock(gitStorageFS)
}
