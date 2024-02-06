package core

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestPreInit(t *testing.T) {
	testconfig.AllowParallelization(t)

	defaultGlobalPermissions := []Permission{
		GlobalVarPermission{permkind.Read, "*"},
		GlobalVarPermission{permkind.Use, "*"},
		GlobalVarPermission{permkind.Create, "*"},
	}

	//register limits
	{
		resetLimitRegistry()
		defer resetLimitRegistry()

		limRegistry.registerLimit("a", TotalLimit, 0)
		limRegistry.registerLimit("b", ByteRateLimit, 0)

	}

	threadLimit := mustGetMinimumNotAutoDepletingCountLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME)

	minLimitA := Limit{
		Name:  "a",
		Kind:  TotalLimit,
		Value: 0,
	}

	minLimitB := Limit{
		Name:  "b",
		Kind:  ByteRateLimit,
		Value: 0,
	}

	//register host definition checking functions
	{
		resetStaticallyCheckHostDefinitionDataFnRegistry()
		defer resetStaticallyCheckHostDefinitionDataFnRegistry()

		RegisterStaticallyCheckHostDefinitionFn("ldb", func(project Project, node parse.Node) (errorMsg string) {
			return ""
		})

		RegisterStaticallyCheckHostDefinitionFn("s3", func(project Project, node parse.Node) (errorMsg string) {
			return ""
		})
	}

	type testCase struct {
		//input
		name                string
		moduleKind          ModuleKind //ok if not set, should be set to the same vale as expectedModuleKind if expectedModuleKind is set
		module              string
		setup               func() error
		teardown            func()
		setupFilesystem     func(fls afs.Filesystem) //called after setup
		parentModule        string
		parentModuleAbsPath string

		//output
		expectedModuleKind           *ModuleKind
		expectedPermissions          []Permission
		expectedLimits               []Limit
		expectedParameters           []ModuleParameter
		expectedResolutions          map[Host]Value
		expectedPreinitFileConfigs   PreinitFiles
		expectedDatabaseConfigs      DatabaseConfigs
		expectedAutoInvocationConfig *AutoInvocationConfig

		//errors
		error                     bool
		expectedParsingError      bool
		errorIs                   error  //optional
		errorContains             string //optional
		expectedStaticCheckErrors []string
		expectedPreinitFileErrors []string
	}

	var testCases = []testCase{
		{
			name: "parsing error in manifest",
			module: `
				manifest {
					permissions: {
						read: {
							%ldb:
						}
					}
				}`,
			error:                true,
			expectedParsingError: true,
			errorIs:              ErrParsingErrorInManifestOrPreinit,
		},
		{
			name: "kind: application",
			module: `
				manifest {
					kind: "application"
				}`,
			expectedLimits:     []Limit{minLimitA, minLimitB, threadLimit},
			expectedModuleKind: utils.New(ApplicationModule),
			moduleKind:         ApplicationModule,
		},
		{
			name: "kind: module kind should be a string literal",
			module: `
				manifest {
					kind: true
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{KIND_SECTION_SHOULD_BE_A_STRING_LITERAL},
		},
		{
			name: "kind: embedded module kinds are not allowed",
			module: `
				manifest {
					kind: "userlthread"
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED},
		},
		{
			name: "kind: invalid module kind",
			module: `
				manifest {
					kind: "?"
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{ErrInvalidModuleKind.Error()},
		},
		{
			name: "parameters: non positional with named pattern",
			module: `
				manifest {
					parameters: {
						name: %str
					}
				}`,
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedParameters: []ModuleParameter{
				{
					positional: false,
					pattern:    STR_PATTERN,
					name:       "name",
					cliArgName: "name",
				},
			},
			error: false,
		},
		{
			name: "parameters: non positional with pattern namespace member",
			module: `
				manifest {
					parameters: {
						node: %inox.node
					}
				}`,
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedParameters: []ModuleParameter{
				{
					positional: false,
					pattern:    ASTNODE_PATTERN,
					name:       "node",
					cliArgName: "node",
				},
			},
			error: false,
		},
		{
			name: "parameters: non positional with description: pattern + default + description",
			module: `
				manifest {
					parameters: {
						name: {
							default: "foo"
							pattern: %str
							description: "..."
						}
					}
				}`,
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedParameters: []ModuleParameter{
				{
					positional:  false,
					pattern:     STR_PATTERN,
					name:        "name",
					cliArgName:  "name",
					defaultVal:  String("foo"),
					description: "...",
				},
			},
		},
		{
			name: "parameters: non positional with description: pattern + char-name",
			module: `
				manifest {
					parameters: {
						name: {
							char-name: 'n'
							pattern: %str
						}
					}
				}`,
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedParameters: []ModuleParameter{
				{
					positional:             false,
					pattern:                STR_PATTERN,
					name:                   "name",
					cliArgName:             "name",
					singleLetterCliArgName: 'n',
				},
			},
		},
		{
			name: "host definition",
			module: `
				manifest {
					host-definitions: :{
						ldb://main : /mydb
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: map[Host]Value{"ldb://main": Path("/mydb")},
			error:               false,
		},
		{
			name: "host definition with object data",
			module: `
				manifest {
					host-definitions: :{
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: map[Host]Value{"s3://database": NewObjectFromMapNoInit(ValMap{
				"bucket":     String("test"),
				"provider":   String("cloudflare"),
				"host":       Host("https://example.com"),
				"access-key": String("x"),
				"secret-key": String("x"),
			})},
			error: false,
		},
		{
			name:                "empty manifest",
			module:              `manifest {}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "read_any_global",
			module: `manifest {
					permissions: { read: {globals: "*"} }
				}`,
			expectedPermissions: []Permission{GlobalVarPermission{permkind.Read, "*"}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create_routine",
			module: `manifest {
					permissions: {
						create: {threads: {}}
					}
				}`,
			expectedPermissions: []Permission{LThreadPermission{permkind.Create}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create_routine",
			module: `manifest {
					permissions: {
						create: {threads: {}}
					}
				}`,
			expectedPermissions: []Permission{LThreadPermission{permkind.Create}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
			expectedPermissions: []Permission{HttpPermission{Kind_: permkind.Read, Entity: URL("https://example.com/")}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "invalid_permission_kind",
			module: `
				const (
					URL = https://example.com/
				)
				manifest {
					permissions: { Read: $$URL}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               true,

			expectedStaticCheckErrors: []string{fmtNotValidPermissionKindName("Read")},
		},
		{
			name: "limits",
			module: `manifest {
					limits: {
						"a": 100ms
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits: []Limit{
				{Name: "a", Kind: TotalLimit, Value: int64(100 * time.Millisecond)},
				minLimitB,
				threadLimit,
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "max_limit_value",
			module: `manifest {
					limits: {
						"a": ` + strconv.FormatInt(MAX_LIMIT_VALUE, 10) + `
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits: []Limit{
				{Name: "a", Kind: TotalLimit, Value: MAX_LIMIT_VALUE},
				minLimitB,
				threadLimit,
			},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "limit_too_high",
			module: `manifest {
					limits: {
						"a": ` + strconv.FormatInt(1+MAX_LIMIT_VALUE, 10) + `
					}
				}`,
			error: true,
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
		{
			name: "read_users_from_database_main",
			module: `manifest {
					permissions: {
						read: {
							%ldb://main/users
						}
					}
				}`,
			expectedPermissions: []Permission{
				DatabasePermission{
					permkind.Read,
					URLPattern("ldb://main/users"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
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
			name: "use named command",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"go": {}
							}
						}
					}
				}`,
			expectedPermissions: []Permission{
				CommandPermission{
					CommandName: String("go"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
		},
		{
			name: "use command with path",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"/usr/local/go/bin/go": {}
							}
						}
					}
				}`,
			expectedPermissions: []Permission{
				CommandPermission{
					CommandName: Path("/usr/local/go/bin/go"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
		},
		{
			name: "use commands matching a prefix path pattern",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"%/usr/local/...": {}
							}
						}
					}
				}`,
			expectedPermissions: []Permission{
				CommandPermission{
					CommandName: PathPattern("/usr/local/..."),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
		},
		{
			name: "use commands matching a globbing path pattern (error)",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"%/usr/local/**/*": {}
							}
						}
					}
				}`,
			error: true,
		},
		{
			name: "use a sub command",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"go": {
									"help": {}
								}
							}
						}
					}
				}`,
			expectedPermissions: []Permission{
				CommandPermission{
					CommandName:         String("go"),
					SubcommandNameChain: []string{"help"},
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
		},
		{
			name: "use a nested sub command",
			module: `
				manifest {
					permissions: {
						use: {
							commands: {
								"go": {
									"help": {
										"x": {}
									}
								}
							}
						}
					}
				}`,
			expectedPermissions: []Permission{
				CommandPermission{
					CommandName:         String("go"),
					SubcommandNameChain: []string{"help", "x"},
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
		},
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
			name: "inclusion import: pattern definition",
			module: `
				preinit {
					import /models.ix
				}

				manifest {
					parameters: {
						name: %username
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/models.ix", []byte(`
					includable-chunk

					pattern username = str
				`), 0600)
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedParameters: []ModuleParameter{
				{
					positional: false,
					pattern:    STR_PATTERN,
					name:       "name",
					cliArgName: "name",
				},
			},
		},
		{
			name: "inclusion import: host alias definition",
			module: `
				preinit {
					import /hosts.ix
				}

				manifest {
					permissions: {
						read: @host/index.html
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/hosts.ix", []byte(`
					includable-chunk

					@host = https://localhost:8080
				`), 0600)
			},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPermissions: []Permission{HttpPermission{Kind_: permkind.Read, Entity: URL("https://localhost:8080/index.html")}},
		},
		{
			name: "inclusion import: forbidden node in chunk",
			module: `
				preinit {
					import /models.ix
				}

				manifest {
					parameters: {
						name: %username
					}
				}`,
			setupFilesystem: func(fls afs.Filesystem) {
				util.WriteFile(fls, "/models.ix", []byte(`
					go do {}
				`), 0600)
			},
			error:         true,
			errorContains: FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT,
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               "/file.txt",
					Parsed:             String("a"),
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
					pattern p = %str("a"+)
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_database",
			module: `manifest {
					databases: {
						main: {
							resource: ldb://main
							resolution-data: nil
						}
					}
				}`,
			expectedPermissions: []Permission{
				DatabasePermission{
					permkind.Read,
					Host("ldb://main"),
				},
				DatabasePermission{
					permkind.Write,
					Host("ldb://main"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Owned:          true,
					Resource:       Host("ldb://main"),
					ResolutionData: Nil,
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
							resolution-data: nil
							expected-schema-update: true
						}
					}
				}`,
			expectedPermissions: []Permission{
				DatabasePermission{
					permkind.Read,
					Host("ldb://main"),
				},
				DatabasePermission{
					permkind.Write,
					Host("ldb://main"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:                 "main",
					Owned:                true,
					Resource:             Host("ldb://main"),
					ResolutionData:       Nil,
					ExpectedSchemaUpdate: true,
				},
			},
			expectedResolutions: nil,
		},
		{
			name: "correct_database_with_assert_schema",
			module: `
				preinit {
					pattern expected-schema = %{
						user: {name: "foo"}
					}
				}
				manifest {
					databases: {
						main: {
							resource: ldb://main
							resolution-data: nil
							assert-schema: %expected-schema
						}
					}
				}`,
			expectedPermissions: []Permission{
				DatabasePermission{
					permkind.Read,
					Host("ldb://main"),
				},
				DatabasePermission{
					permkind.Write,
					Host("ldb://main"),
				},
			},
			expectedLimits: []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Owned:          true,
					Resource:       Host("ldb://main"),
					ResolutionData: Nil,
					ExpectedSchema: NewInexactObjectPattern([]ObjectPatternEntry{
						{
							Name: "user",
							Pattern: NewInexactObjectPattern([]ObjectPatternEntry{
								{Name: "name", Pattern: NewExactStringPattern("foo")},
							}),
						},
					}),
				},
			},
		},
		{
			name: "correct database with invalid assert_schema",
			module: `
				preinit {
					pattern expected-schema = %{
						user: str # no loading function
					}
				}
				manifest {
					databases: {
						main: {
							resource: ldb://main
							resolution-data: nil
							assert-schema: %expected-schema
						}
					}
				}`,
			error:   true,
			errorIs: ErrNoLoadFreeEntityFnRegistered,
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
			expectedStaticCheckErrors: []string{DATABASES__DB_RESOLUTION_DATA_ONLY_NIL_AND_PATHS_SUPPORTED},
		},
		{
			name: "database_with_invalid_expected_schema_udapte_value",
			module: `manifest {
					databases: {
						main: {
							resource: ldb://main
							resolution-data: nil
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
							resolution-data: nil
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
						resolution-data: nil
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest {
					databases: /main.ix
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Nil,
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
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
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
		{
			name: "invalid_invocation_section_missing_dbs_section",
			module: `manifest {
					invocation: {
						on-added-element: ldb://main/users
					}
				}`,

			setup: func() error {
				resetStaticallyCheckDbResolutionDataFnRegistry()

				RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node, p Project) (errorMsg string) {
					return ""
				})

				return nil
			},
			teardown: func() {
				resetStaticallyCheckDbResolutionDataFnRegistry()
			},
			error:                     true,
			expectedStaticCheckErrors: []string{THE_DATABASES_SECTION_SHOULD_BE_PRESENT},
		},
		{
			name: "invocation_section_with_added_elem_and_dbs_section",
			parentModule: `manifest {
				databases: {
					main: {
						resource: ldb://main
						resolution-data: nil
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest {
					databases: /main.ix
					invocation: {
						on-added-element: ldb://main/users
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Nil,
				},
			},
			expectedAutoInvocationConfig: &AutoInvocationConfig{
				OnAddedElement: "ldb://main/users",
			},
			setup: func() error {
				resetStaticallyCheckDbResolutionDataFnRegistry()

				RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node, p Project) (errorMsg string) {
					return ""
				})

				return nil
			},
			teardown: func() {
				resetStaticallyCheckDbResolutionDataFnRegistry()
			},
		},

		{
			name: "invalid_invocation_section_non_bool_async",
			parentModule: `manifest {
				databases: {
					main: {
						resource: ldb://main
						resolution-data: nil
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest {
					databases: /main.ix
					invocation: {
						on-added-element: ldb://main/users
						async: 1
					}
				}`,
			expectedPermissions:       []Permission{},
			expectedLimits:            []Limit{},
			error:                     true,
			expectedStaticCheckErrors: []string{A_BOOL_LIT_IS_EXPECTED},

			setup: func() error {
				resetStaticallyCheckDbResolutionDataFnRegistry()

				RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node, p Project) (errorMsg string) {
					return ""
				})

				return nil
			},
			teardown: func() {
				resetStaticallyCheckDbResolutionDataFnRegistry()
			},
		},

		{
			name: "invalid_invocation_section_not_defined_db",
			parentModule: `manifest {
				databases: {
					main: {
						resource: ldb://main
						resolution-data: nil
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest {
					databases: /main.ix
					invocation: {
						on-added-element: ldb://notdefined/users
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			error:               true,
			errorIs:             ErrURLNotCorrespondingToDefinedDB,

			setup: func() error {
				resetStaticallyCheckDbResolutionDataFnRegistry()

				RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node, p Project) (errorMsg string) {
					return ""
				})

				return nil
			},
			teardown: func() {
				resetStaticallyCheckDbResolutionDataFnRegistry()
			},
		},

		{
			name: "invocation_section_with_added_elem_and_async",
			parentModule: `manifest {
				databases: {
					main: {
						resource: ldb://main
						resolution-data: nil
					}
				}
			}`,
			parentModuleAbsPath: "/main.ix",
			module: `manifest {
					databases: /main.ix
					invocation: {
						on-added-element: ldb://main/users
						async: true
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedDatabaseConfigs: DatabaseConfigs{
				{
					Name:           "main",
					Resource:       Host("ldb://main"),
					ResolutionData: Nil,
				},
			},
			expectedAutoInvocationConfig: &AutoInvocationConfig{
				OnAddedElement: "ldb://main/users",
				Async:          true,
			},
			setup: func() error {
				resetStaticallyCheckDbResolutionDataFnRegistry()

				RegisterStaticallyCheckDbResolutionDataFn("ldb", func(node parse.Node, p Project) (errorMsg string) {
					return ""
				})

				return nil
			},
			teardown: func() {
				resetStaticallyCheckDbResolutionDataFnRegistry()
			},
		},

		//check the parameters section is forbidden in most modules.

		{
			name:       "the parameters section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the parameters section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//check the databases section is forbidden in most modules.

		{
			name:       "the databases section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the databases section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the databases section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the databases section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the databases section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//check the parameters section is forbidden in most modules.

		{
			name:       "the parameters section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the parameters section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the parameters section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//check the invocation section is forbidden in most modules.

		{
			name:       "the invocation section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the invocation section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the invocation section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the invocation section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the invocation section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//check the preinit-files section is forbidden in most modules.

		{
			name:       "the preinit-files section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the preinit-files section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the preinit-files section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the preinit-files section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the preinit-files section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//check the env section is forbidden in most modules.

		{
			name:       "the env section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", SpecModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the env section is not allowed in lifetime job modules",
			moduleKind: LifetimeJobModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", LifetimeJobModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the env section is not allowed in lthread modules",
			moduleKind: UserLThreadModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", UserLThreadModule)},
			expectedLimits:            []Limit{},
		},

		{
			name:       "the env section is not allowed in testsuite modules",
			moduleKind: TestSuiteModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", TestSuiteModule)},
			expectedLimits:            []Limit{},
		},
		{
			name:       "the env section is not allowed in testcase modules",
			moduleKind: TestCaseModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{fmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", TestCaseModule)},
			expectedLimits:            []Limit{},
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

			if testCase.setup != nil {
				err := testCase.setup()
				if !assert.NoError(t, err) {
					return
				}
			}

			if testCase.teardown != nil {
				defer testCase.teardown()
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
					MainChunk:        parse.NewParsedChunkSource(chunk, srcFile),
					ManifestTemplate: chunk.Manifest,
				}

				parentState = NewGlobalState(NewContext(ContextConfig{
					DoNotSpawnDoneGoroutine: true,
				}))
				defer parentState.Ctx.CancelGracefully()
				parentState.Module = mod
				parentState.MainState = parentState

				start := time.Now()
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

				//PreInit should be fast
				if !assert.Less(t, time.Since(start), time.Millisecond) {
					return
				}

				parentState.Manifest = manifest
			}

			chunk, err := parse.ParseChunk(testCase.module, "<chunk>")

			if !testCase.expectedParsingError && !assert.NoError(t, err) {
				return
			}

			mod := &Module{
				MainChunk: parse.NewParsedChunkSource(chunk,
					parse.SourceFile{
						NameString:             "/main.ix",
						UserFriendlyNameString: "/main.ix",
						Resource:               "/main.ix",
						ResourceDir:            "/",
						IsResourceURL:          false,
						CodeString:             testCase.module,
					},
				),
				ModuleKind:            testCase.moduleKind,
				ManifestTemplate:      chunk.Manifest,
				InclusionStatementMap: map[*parse.InclusionImportStatement]*IncludedChunk{},
				IncludedChunkMap:      map[string]*IncludedChunk{},
			}

			{
				ctx := NewContext(ContextConfig{
					Permissions: []Permission{
						FilesystemPermission{Kind_: permkind.Read, Entity: PathPattern("/...")},
					},
					DoNotSpawnDoneGoroutine: true,
					Filesystem:              fls,
				})
				ParseLocalIncludedFiles(mod, ctx, fls, false)
				ctx.CancelGracefully()
			}

			start := time.Now()
			manifest, _, staticCheckErrors, err := mod.PreInit(PreinitArgs{
				PreinitFilesystem:     fls,
				Filesystem:            fls,
				GlobalConsts:          chunk.GlobalConstantDeclarations,
				PreinitStatement:      chunk.Preinit,
				RunningState:          nil,
				ParentState:           parentState,
				AddDefaultPermissions: true,
			})

			//PreInit should be fast
			if !assert.Less(t, time.Since(start), 2*time.Millisecond) {
				return
			}

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
					assert.Fail(t, fmt.Sprintf("expected static check errors to contain an error with the substring: %s", expected))
				}

				for _, err := range remainingErrors {
					assert.Fail(t, fmt.Sprintf("the following static check error was not expected: %s", err.Error()))
				}
			}

			if testCase.error {
				if !assert.Error(t, err) {
					return
				}
				if testCase.errorIs != nil {
					if !assert.ErrorIs(t, err, testCase.errorIs) {
						return
					}
				}
				if testCase.errorContains != "" {
					if !assert.ErrorContains(t, err, testCase.errorContains) {
						return
					}
				}
			} else {
				if !assert.NoError(t, err) {
					return
				}
			}

			if manifest != nil {
				if testCase.expectedParameters != nil {
					params := slices.Clone(manifest.Parameters.positional)
					params = append(params, manifest.Parameters.others...)
					assert.EqualValues(t, testCase.expectedParameters, params)
				}

				if testCase.expectedModuleKind != nil {
					assert.EqualValues(t, *testCase.expectedModuleKind, manifest.explicitModuleKind)
				}
				assert.EqualValues(t, testCase.expectedPermissions, manifest.RequiredPermissions)
				assert.ElementsMatch(t, testCase.expectedLimits, manifest.Limits)
				assert.EqualValues(t, testCase.expectedResolutions, manifest.HostDefinitions)
				assert.EqualValues(t, testCase.expectedAutoInvocationConfig, manifest.AutoInvocation)

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
