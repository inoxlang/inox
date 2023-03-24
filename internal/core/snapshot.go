package internal

import (
	"fmt"
	"time"
)

type Snapshot struct {
	date Date
	repr ValueRepresentation
}

func (s *Snapshot) Date() Date {
	return s.date
}

func TakeSnapshotOfSimpleValue(ctx *Context, v Value) (*Snapshot, error) {
	repr, err := GetRepresentationWithConfig(v, &ReprConfig{allVisible: true}, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take snapshot of value of type %T: %w", v, err)
	}
	return &Snapshot{
		date: Date(time.Now()),
		repr: repr,
	}, nil
}

func (s *Snapshot) InstantiateValue(ctx *Context) (Value, error) {
	v, err := ParseRepr(ctx, s.repr)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate snapshot's value: %w", err)
	}
	return v, nil
}

func (s *Snapshot) WithChangeApplied(ctx *Context, c Change) (*Snapshot, error) {
	v, err := s.InstantiateValue(ctx)
	if err != nil {
		return nil, err
	}

	snapshotDate := time.Time(s.date)
	changeDate := time.Time(c.date)

	if snapshotDate.After(changeDate) {
		return nil, fmt.Errorf("cannot apply a change (date: %s) on a snapshot that is more recent than the change: %s", changeDate, snapshotDate)
	}

	if err := c.mutation.ApplyTo(ctx, v); err != nil {
		return nil, err
	}

	repr, err := GetRepresentationWithConfig(v, &ReprConfig{allVisible: true}, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to take snapshot of value: %w", err)
	}

	return &Snapshot{
		date: c.date,
		repr: repr,
	}, nil
}
