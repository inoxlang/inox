package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestPreInit(t *testing.T) {

	defaultGlobalPermissions := []Permission{
		GlobalVarPermission{permkind.Read, "*"},
		GlobalVarPermission{permkind.Use, "*"},
		GlobalVarPermission{permkind.Create, "*"},
	}
	LimRegistry.RegisterLimitation("a", TotalLimitation, 0)
	LimRegistry.RegisterLimitation("b", ByteRateLimitation, 0)

	RegisterStaticallyCheckHostResolutionDataFn("ldb", func(node parse.Node) (errorMsg string) {
		return ""
	})

	RegisterStaticallyCheckHostResolutionDataFn("s3", func(node parse.Node) (errorMsg string) {
		return ""
	})

	type testCase struct {
		//input
		name                string
		module              string
		setupFilesystem     func(fls afs.Filesystem)
		parentModule        string
		parentModuleAbsPath string

		//output
		expectedPermissions        []Permission
		expectedLimitations        []Limitation
		expectedResolutions        map[Host]Value
		expectedPreinitFileConfigs PreinitFiles
		expectedDatabaseConfigs    DatabaseConfigs

		//errors
		error                     bool
		expectedStaticCheckErrors []string
		expectedPreinitFileErrors []string
	}

	var testCases = []testCase{
		{
			name: "host resolution",
			module: `
				manifest {
					host-resolution: :{
						ldb://main : /mydb
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: map[Host]Value{"ldb://main": Path("/mydb")},
			error:               false,
		},
		{
			name: "host resolution with object data",
			module: `
				manifest {
					host-resolution: :{
						s3://database : {
							bucket: "test"
							provider: "cloudflare"
							host: https://example.com
							access-key: "x"
							secret-key: "x"
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: map[Host]Value{"s3://database": NewObjectFromMapNoInit(ValMap{
				"bucket":     Str("test"),
				"provider":   Str("cloudflare"),
				"host":       Host("https://example.com"),
				"access-key": Str("x"),
				"secret-key": Str("x"),
			})},
			error: false,
		},
		{
			name:                "empty manifest",
			module:              `manifest {}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "read_any_global",
			module: `manifest {
					permissions: { read: {globals: "*"} }
				}`,
			expectedPermissions: []Permission{GlobalVarPermission{permkind.Read, "*"}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create_routine",
			module: `manifest {
					permissions: {
						create: {routines: {}}
					}
				}`,
			expectedPermissions: []Permission{RoutinePermission{permkind.Create}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create_routine",
			module: `manifest {
					permissions: {
						create: {routines: {}}
					}
				}`,
			expectedPermissions: []Permission{RoutinePermission{permkind.Create}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "read_@const_var",
			module: `
				const (
					URL = https://example.com/
				)
				manifest {
					permissions: { read: $$URL}
				}`,
			expectedPermissions: []Permission{HttpPermission{permkind.Read, URL("https://example.com/")}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "limitations",
			module: `manifest {
					limits: {
						"a": 100ms
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{
				{Name: "a", Kind: TotalLimitation, Value: int64(100 * time.Millisecond)},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "host_with_unsupported_scheme",
			module: `manifest {
					permissions: {
						read: mem://a.com
					}
				}`,
			error: true,
		},
		{
			name: "host_pattern_with_unsupported_scheme",
			module: `manifest {
					permissions: { read: %ws://*.com }
				}`,
			error: true,
		},
		{
			name: "dns",
			module: `manifest {
					permissions: {
						read: {
							dns: %://**.com
						}
					}
				}`,
			expectedPermissions: []Permission{
				DNSPermission{permkind.Read, HostPattern("://**.com")},
			},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "dns_host_pattern_literal_with_scheme",
			module: `manifest {
					permissions: {
						read: {
							dns: %https://**.com
						}
					}
				}`,
			error: true,
		},
		// {
		// 	name: "see email addresses",
		// 	inputModule: `manifest {
		// 			permissions: {
		// 				see: { values: %emailaddr }
		// 			}
		// 		}`,
		// 	//error: true,
		// },
		// {
		// 	name: "see email addresses & ints",
		// 	inputModule: `manifest {
		// 		permissions: {
		// 			see: { values: [%emailaddr, %int] }
		// 		}
		// 	}`,
		// 	//error: true,
		// },
		{
			name: "invalid node type in preinit block",
			module: `
			preinit {
				go {} do {}
			}

			manifest {
				permissions: {}
			}`,
			expectedPermissions: []Permission{
				ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN},
				ValueVisibilityPermission{Pattern: INT_PATTERN},
			},
			expectedStaticCheckErrors: []string{ErrForbiddenNodeinPreinit.Error()},
			error:                     true,
		},
		{
			name: "invalid value for permissions section",
			module: `
			manifest {
				permissions: 1
			}`,
			expectedPermissions: []Permission{
				ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN},
				ValueVisibilityPermission{Pattern: INT_PATTERN},
			},
			expectedStaticCheckErrors: []string{PERMS_SECTION_SHOULD_BE_AN_OBJECT},
			error:                     true,
		},
		{
			name: "empty_preinit_files",
			module: `manifest {
					preinit-files: {}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_preinit_file",
			module: `manifest {
					preinit-files: {
						F: {
							path: /file.txt
							pattern: %str
						}
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/file.txt", nil, 0o600)
			},
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               "/file.txt",
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file.txt")),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_preinit_file",
			module: `manifest {
					preinit-files: {
						F: {
							path: /file.txt
							pattern: %str
						}
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/file.txt", []byte("a"), 0o600)
			},
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               "/file.txt",
					Parsed:             Str("a"),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file.txt")),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_preinit_file_but_not_existing",
			module: `manifest {
					preinit-files: {
						F: {
							path: /file.txt
							pattern: %str
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               "/file.txt",
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file.txt")),
				},
			},
			expectedResolutions:       nil,
			expectedPreinitFileErrors: []string{os.ErrNotExist.Error()},
			error:                     true,
		},
		{
			name: "correct_preinit_file_but_content_not_matching_pattern",
			module: `
				preinit {
					%p = %str("a"+)
				}
				manifest {
					preinit-files: {
						F: {
							path: /file.txt
							pattern: %p
						}
					}
				}
			`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/file.txt", nil, 0o600)
			},
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               "/file.txt",
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file.txt")),
				},
			},
			expectedResolutions:       nil,
			expectedPreinitFileErrors: []string{ErrInvalidInputString.Error()},
			error:                     true,
		},
		{
			name: "several_correct_preinit_files",
			module: `manifest {
					preinit-files: {
						F1: {
							path: /file1.txt
							pattern: %str
						}
						F2: {
							path: /file2.txt
							pattern: %str
						}
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/file1.txt", nil, 0o600)
				util.WriteFile(fls, "/file2.txt", nil, 0o600)
			},
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F1",
					Path:               "/file1.txt",
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file1.txt")),
				},
				{
					Name:               "F2",
					Path:               "/file2.txt",
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path("/file2.txt")),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "preinit-files_section_should_be_an_object",
			module: `manifest { 
					preinit-files: 1
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT},
		},
		{
			name: "empty_databases",
			module: `manifest { 
					databases: {}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_database",
			module: `manifest { 
					databases: {
						main: {
							resource: ldb://main
							resolution-data: /tmp/mydb/
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Path("/tmp/mydb/"),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_database_with_expected_schema_update",
			module: `manifest { 
					databases: {
						main: {
							resource: ldb://main
							resolution-data: /tmp/mydb/
							expected-schema-update: true
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:                 "main",
					Resource:             Host("ldb://main"),
					ResolutionData:       Path("/tmp/mydb/"),
					ExpectedSchemaUpdate: true,
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "database_with_invalid_resource",
			module: `manifest { 
					databases: {
						main: {
							resource: 1
							resolution-data: /db/
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_RESOURCE_SHOULD_BE_HOST_OR_URL},
		},
		{
			name: "database_with_invalid_resolution_data",
			module: `manifest { 
					databases: {
						main: {
							resource: ldb://main
							resolution-data: 1
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_RESOLUTION_DATA_ONLY_PATHS_SUPPORTED},
		},
		{
			name: "database_with_invalid_expected_schema_udapte_value",
			module: `manifest { 
					databases: {
						main: {
							resource: ldb://main
							resolution-data: /db/
							expected-schema-update: 1
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_EXPECTED_SCHEMA_UPDATE_SHOULD_BE_BOOL_LIT},
		},
		{
			name: "database_with_missing_resource",
			module: `manifest { 
					databases: {
						main: {
							resolution-data: /db/
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtMissingPropInDatabaseDescription(MANIFEST_DATABASE__RESOURCE_PROP_NAME, "main")},
		},
		{
			name: "database_with_missing_resolution_data",
			module: `manifest { 
					databases: {
						main: {
							resource: ldb://main
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtMissingPropInDatabaseDescription(MANIFEST_DATABASE__RESOLUTION_DATA_PROP_NAME, "main")},
		},
		{
			name: "database_description_should_be_an_object",
			module: `manifest { 
					databases: {
						main: 1
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_CONFIG_SHOULD_BE_AN_OBJECT},
		},
		{
			name: "databases_section_should_be_an_object",
			module: `manifest { 
					databases: 1
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES_SECTION_SHOULD_BE_AN_OBJECT_OR_ABS_PATH},
		},

		{
			name: "databases_path_value",
			parentModule: `manifest { 
				databases: {
					main: {
						resource: ldb://main
						resolution-data: /tmp/mydb/
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest { 
					databases: /main.ix
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Path("/tmp/mydb/"),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "databases_host_value",
			parentModule: `manifest { 
				databases: {
					main: {
						resource: ldb://main
						resolution-data: s3://database
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest { 
					databases: /main.ix
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Host("s3://database"),
				},
			},
			expectedResolutions: nil,
			error:               false,
		},

		//TODO: improve tests.
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "read_any_global" {
				testCase.expectedPermissions =
					append(testCase.expectedPermissions, GlobalVarPermission{permkind.Use, "*"}, GlobalVarPermission{permkind.Create, "*"})
			} else {
				testCase.expectedPermissions = append(testCase.expectedPermissions, defaultGlobalPermissions...)
			}

			if testCase.expectedResolutions == nil {
				testCase.expectedResolutions = map[Host]Value{}
			}

			fls := newMemFilesystem()
			if testCase.setupFilesystem != nil {
				testCase.setupFilesystem(fls)
			}

			var parentState *GlobalState

			if testCase.parentModule != "" {
				chunk := parse.MustParseChunk(testCase.parentModule)

				srcFile := parse.SourceFile{
					NameString:  testCase.parentModuleAbsPath,
					Resource:    testCase.parentModuleAbsPath,
					ResourceDir: filepath.Dir(testCase.parentModuleAbsPath),
					CodeString:  testCase.parentModule,
				}

				mod := &Module{
					MainChunk:        parse.NewParsedChunk(chunk, srcFile),
					ManifestTemplate: chunk.Manifest,
				}

				parentState = NewGlobalState(NewContext(ContextConfig{}))
				parentState.Module = mod
				parentState.MainState = parentState

				manifest, _, _, err := mod.PreInit(PreinitArgs{
					PreinitFilesystem:     fls,
					GlobalConsts:          chunk.GlobalConstantDeclarations,
					PreinitStatement:      chunk.Preinit,
					RunningState:          nil,
					AddDefaultPermissions: true,
				})

				if !assert.NoError(t, err) {
					return
				}

				parentState.Manifest = manifest
			}

			chunk := parse.MustParseChunk(testCase.module)

			mod := &Module{
				MainChunk: parse.NewParsedChunk(chunk, parse.InMemorySource{
					NameString: "test",
					CodeString: testCase.module,
				}),
				ManifestTemplate: chunk.Manifest,
			}

			manifest, _, staticCheckErrors, err := mod.PreInit(PreinitArgs{
				PreinitFilesystem:     fls,
				GlobalConsts:          chunk.GlobalConstantDeclarations,
				PreinitStatement:      chunk.Preinit,
				RunningState:          nil,
				ParentState:           parentState,
				AddDefaultPermissions: true,
			})

			if len(testCase.expectedStaticCheckErrors) > 0 {
				remainingErrors := map[string]error{}
				for _, err := range staticCheckErrors {
					remainingErrors[err.Error()] = err
				}

			outer:
				for _, expected := range testCase.expectedStaticCheckErrors {
					for _, err := range staticCheckErrors {
						if strings.Contains(err.Error(), expected) {
							delete(remainingErrors, err.Error())
							continue outer
						}
					}
					assert.Fail(t, fmt.Sprintf("expected static check errors to contain the error: %s", expected))
				}

				for _, err := range remainingErrors {
					assert.Fail(t, fmt.Sprintf("the following static check error was not expected: %s", err.Error()))
				}
			}

			if testCase.error {
				if !assert.Error(t, err) {
					return
				}
			} else {
				if !assert.NoError(t, err) {
					return
				}
			}

			if manifest != nil {
				assert.EqualValues(t, testCase.expectedPermissions, manifest.RequiredPermissions)
				assert.EqualValues(t, testCase.expectedLimitations, manifest.Limitations)
				assert.EqualValues(t, testCase.expectedResolutions, manifest.HostResolutions)

				if testCase.expectedPreinitFileErrors == nil {
					for _, preinitFile := range manifest.PreinitFiles {
						assert.NoError(t, preinitFile.ReadParseError, "error for preinit file "+preinitFile.Path)
					}
				} else {
					if len(manifest.PreinitFiles) != len(testCase.expectedPreinitFileErrors) {
						assert.Fail(t, fmt.Sprintf("mismatch between length of preinit files (%d) and expected preinit file errors (%d)",
							len(manifest.PreinitFiles), len(testCase.expectedPreinitFileErrors)))
					} else {
						for i, preinitFile := range manifest.PreinitFiles {
							if testCase.expectedPreinitFileErrors[i] != "" {
								assert.ErrorContains(t, preinitFile.ReadParseError, testCase.expectedPreinitFileErrors[i])
							}
						}
					}
				}

				if testCase.expectedDatabaseConfigs != nil {

					if len(manifest.Databases) != len(testCase.expectedDatabaseConfigs) {
						assert.Fail(t, fmt.Sprintf("mismatch between number of databases (%d) and number of expected databases (%d)",
							len(manifest.Databases), len(testCase.expectedDatabaseConfigs)))
						return
					}

					for i, db := range manifest.Databases {
						assert.Equal(t, testCase.expectedDatabaseConfigs[i], db)
					}
				}
			}
		})
	}

}
