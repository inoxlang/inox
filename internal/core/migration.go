package core

import "path/filepath"

// TODO: improve name
type MigrationAwarePattern interface {
	Pattern
	GetMigrationOperations(next Pattern, pseudoPath string) ([]MigrationOp, error)
}

type MigrationOp interface {
	PseudPath()
}

type migrationMixin struct {
	pseudoPath string
}

func (m migrationMixin) PseudPath() {

}

type ReplacementMigrationOp struct {
	Current, Next Pattern
	migrationMixin
}

type RemovalMigrationOp struct {
	Value Pattern
	migrationMixin
}

type NillableInitializationMigrationOp struct {
	Value Pattern
	migrationMixin
}

type InclusionMigrationOp struct {
	Value    Pattern
	Optional bool
	migrationMixin
}

func (patt *ObjectPattern) GetMigrationOperations(next Pattern, pseudoPath string) (migrations []MigrationOp, _ error) {
	nextObject, ok := next.(*ObjectPattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, migrationMixin: migrationMixin{pseudoPath: pseudoPath}}}, nil
	}

	if patt.entryPatterns == nil {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, migrationMixin: migrationMixin{pseudoPath: pseudoPath}}}, nil
	}

	if nextObject.entryPatterns == nil {
		return nil, nil
	}

	for propName, propPattern := range patt.entryPatterns {
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := patt.optionalEntries[propName]
		_, isOptionalInOther := nextObject.optionalEntries[propName]

		nextPropPattern, presentInOther := nextObject.entryPatterns[propName]

		if !presentInOther {
			migrations = append(migrations, RemovalMigrationOp{
				Value:          propPattern,
				migrationMixin: migrationMixin{propPseudoPath},
			})
			continue
		}

		list, err := GetMigrationOperations(propPattern, nextPropPattern, propPseudoPath)
		if err != nil {
			return nil, err
		}

		if len(list) == 0 && isOptional && !isOptionalInOther {
			list = append(list, NillableInitializationMigrationOp{
				Value:          propPattern,
				migrationMixin: migrationMixin{propPseudoPath},
			})
		}
		migrations = append(migrations, list...)
	}

	for propName, nextPropPattern := range nextObject.entryPatterns {
		_, presentInCurrent := patt.entryPatterns[propName]

		if presentInCurrent {
			//already handled
			continue
		}
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := nextObject.optionalEntries[propName]

		migrations = append(migrations, InclusionMigrationOp{
			Value:          nextPropPattern,
			Optional:       isOptional,
			migrationMixin: migrationMixin{propPseudoPath},
		})
	}

	return migrations, nil
}

func (patt *RecordPattern) GetMigrationOperations(next Pattern, pseudoPath string) (migrations []MigrationOp, _ error) {
	nextRecord, ok := next.(*RecordPattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, migrationMixin: migrationMixin{pseudoPath: pseudoPath}}}, nil
	}

	if patt.entryPatterns == nil {
		return []MigrationOp{ReplacementMigrationOp{Current: patt, Next: next, migrationMixin: migrationMixin{pseudoPath: pseudoPath}}}, nil
	}

	if nextRecord.entryPatterns == nil {
		return nil, nil
	}

	for propName, propPattern := range patt.entryPatterns {
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := patt.optionalEntries[propName]
		_, isOptionalInOther := nextRecord.optionalEntries[propName]

		nextPropPattern, presentInOther := nextRecord.entryPatterns[propName]

		if !presentInOther {
			migrations = append(migrations, RemovalMigrationOp{
				Value:          propPattern,
				migrationMixin: migrationMixin{propPseudoPath},
			})
			continue
		}

		list, err := GetMigrationOperations(propPattern, nextPropPattern, propPseudoPath)
		if err != nil {
			return nil, err
		}

		if len(list) == 0 && isOptional && !isOptionalInOther {
			list = append(list, NillableInitializationMigrationOp{
				Value:          propPattern,
				migrationMixin: migrationMixin{propPseudoPath},
			})
		}
		migrations = append(migrations, list...)
	}

	for propName, nextPropPattern := range nextRecord.entryPatterns {
		_, presentInCurrent := patt.entryPatterns[propName]

		if presentInCurrent {
			//already handled
			continue
		}
		propPseudoPath := filepath.Join(pseudoPath, propName)
		_, isOptional := nextRecord.optionalEntries[propName]

		migrations = append(migrations, InclusionMigrationOp{
			Value:          nextPropPattern,
			Optional:       isOptional,
			migrationMixin: migrationMixin{propPseudoPath},
		})
	}

	return migrations, nil
}

func GetMigrationOperations(current, next Pattern, pseudoPath string) ([]MigrationOp, error) {
	if current == next {
		return nil, nil
	}

	m1, ok := current.(MigrationAwarePattern)
	if !ok {
		return []MigrationOp{ReplacementMigrationOp{Current: current, Next: next, migrationMixin: migrationMixin{pseudoPath: pseudoPath}}}, nil
	}

	return m1.GetMigrationOperations(next, pseudoPath)
}
