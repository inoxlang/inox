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

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/sourcecode"

	"github.com/inoxlang/inox/internal/core/inoxmod"
	"github.com/inoxlang/inox/internal/core/limitbase"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/text"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/testconfig"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestPreInit(t *testing.T) {
	testconfig.AllowParallelization(t)

	defaultGlobalPermissions := []Permission{
		GlobalVarPermission{permbase.Read, "*"},
		GlobalVarPermission{permbase.Use, "*"},
		GlobalVarPermission{permbase.Create, "*"},
	}

	//register limits
	{
		limitbase.ResetLimitRegistry()
		defer limitbase.ResetLimitRegistry()

		limitbase.RegisterLimit("a", TotalLimit, 0)
		limitbase.RegisterLimit("b", ByteRateLimit, 0)

	}

	const MAIN_MODULE_NAME = "main.ix"

	threadLimit := limitbase.MustGetMinimumNotAutoDepletingCountLimit(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME)

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
		staticcheck.ResetHostDefinitionDataCheckFnRegistry()
		defer staticcheck.ResetHostDefinitionDataCheckFnRegistry()

		RegisterStaticallyCheckHostDefinitionFn("s3", func(node ast.Node) (errorMsg string) {
			return ""
		})
	}

	//files used in several tests
	preinitFile1Path := filepath.Join(t.TempDir(), "file1.txt")
	preinitFile2Path := filepath.Join(t.TempDir(), "file2.txt")
	nonExistingFilePath := filepath.Join(t.TempDir(), "non_existing.txt")

	modelsFilePath := filepath.Join(t.TempDir(), "models.ix")
	invalidIncludabileFilePath := filepath.Join(t.TempDir(), "invalid.ix")

	os.WriteFile(preinitFile1Path, []byte{'1'}, 0600)
	os.WriteFile(preinitFile2Path, []byte{'2'}, 0600)

	os.WriteFile(modelsFilePath, []byte(`
		includable-file

		pattern username = str
	
	`), 0600)

	os.WriteFile(invalidIncludabileFilePath, []byte(`go do {}`), 0600)

	type testCase struct {
		//input
		name                string
		moduleKind          ModuleKind //ok if not set, should be set to the same vale as expectedModuleKind if expectedModuleKind is set
		module              string
		additionalGlobals   map[string]Value
		setup               func() error
		teardown            func()
		parentModule        string
		parentModuleAbsPath string

		//output
		expectedModuleKind           *ModuleKind
		expectedPermissions          []Permission
		expectedLimits               []Limit
		expectedParameters           []ModuleParameter
		expectedResolutions          map[Host]Value
		expectedPreinitFileConfigs   PreinitFiles
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
			name: "kind: unspecified is not allowed to be specified in the manifest",
			module: `
				manifest {
					kind: "unspecified"
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{text.THE_UNSPECIFIED_MOD_KIND_NAME_CANNOT_BE_USED_IN_THE_MANIFEST},
		},
		{
			name: "kind: module kind should be a string literal",
			module: `
				manifest {
					kind: true
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{text.KIND_SECTION_SHOULD_BE_A_STRING_LITERAL},
		},
		{
			name: "kind: embedded module kinds are not allowed",
			module: `
				manifest {
					kind: "userlthread"
				}`,
			expectedLimits:            []Limit{minLimitA, minLimitB, threadLimit},
			error:                     true,
			expectedStaticCheckErrors: []string{text.INVALID_KIND_SECTION_EMBEDDED_MOD_KINDS_NOT_ALLOWED},
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
			name:       "the kind specified in the manifest should not conflict with the inferred module kind: spec module and 'application' specified",
			moduleKind: SpecModule,
			module: `
				manifest {
					kind: "application"
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.MOD_KIND_SPECIFIED_IN_MANIFEST_SHOULD_BE_SPEC_OR_SHOULD_BE_OMITTED},
			expectedLimits:            []Limit{},
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
			name: "methods of global constants are allowed to be called",
			module: `
				const (
					PATH_A = /a/
					PATH_B = PATH_A.join(./b)
				)
				manifest {
					permissions: { read: PATH_B }
				}`,
			expectedPermissions: []Permission{FilesystemPermission{Kind_: permbase.Read, Entity: Path("/a/b")}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
		},
		{
			name: "additional globals are accessible by the manifest",
			module: `
				manifest {
					permissions: { read: PATH }
				}`,
			expectedPermissions: []Permission{FilesystemPermission{Kind_: permbase.Read, Entity: Path(preinitFile1Path)}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			additionalGlobals:   map[string]Value{"PATH": Path(preinitFile1Path)},
		},
		{
			name: "additional globals are accessible from within the preinit statement",
			module: `
				preinit {
					pattern p = $PATH_PATTERN
				}
				manifest {
					permissions: { read: %p }
				}`,
			expectedPermissions: []Permission{FilesystemPermission{Kind_: permbase.Read, Entity: PathPattern("/...")}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			additionalGlobals:   map[string]Value{"PATH_PATTERN": PathPattern("/...")},
		},
		{
			name: "host definition",
			module: `
				manifest {
					host-definitions: :{
						s3://main : /bucket
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: map[Host]Value{"s3://main": Path("/bucket")},
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
			name: "read any global",
			module: `manifest {
					permissions: { read: {globals: "*"} }
				}`,
			expectedPermissions: []Permission{GlobalVarPermission{permbase.Read, "*"}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create routine",
			module: `manifest {
					permissions: {
						create: {threads: {}}
					}
				}`,
			expectedPermissions: []Permission{LThreadPermission{permbase.Create}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create routine",
			module: `manifest {
					permissions: {
						create: {threads: {}}
					}
				}`,
			expectedPermissions: []Permission{LThreadPermission{permbase.Create}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "read const var",
			module: `
				const (
					URL = https://example.com/
				)
				manifest {
					permissions: { read: $URL}
				}`,
			expectedPermissions: []Permission{HttpPermission{Kind_: permbase.Read, Entity: URL("https://example.com/")}},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "invalid permission kind",
			module: `
				const (
					URL = https://example.com/
				)
				manifest {
					permissions: { Read: $URL}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedResolutions: nil,
			error:               true,

			expectedStaticCheckErrors: []string{text.FmtNotValidPermissionKindName("Read")},
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
				DNSPermission{permbase.Read, HostPattern("://**.com")},
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
		// {
		// 	name: "see email addresses",
		// 	inputModule: `manifest {
		// 			permissions: {
		// 				see: { values: %email-address }
		// 			}
		// 		}`,
		// 	//error: true,
		// },
		// {
		// 	name: "see email addresses & ints",
		// 	inputModule: `manifest {
		// 		permissions: {
		// 			see: { values: [%email-address, %int] }
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
			expectedStaticCheckErrors: []string{staticcheck.ErrForbiddenNodeinPreinit.Error()},
			error:                     true,
		},
		{
			name: "inclusion import: pattern definition",
			module: `
				preinit {
					import ` + modelsFilePath + `
				}

				manifest {
					parameters: {
						name: %username
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
		},
		{
			name: "inclusion import: forbidden node in chunk",
			module: `
				preinit {
					import ` + invalidIncludabileFilePath + `
				}

				manifest {}`,
			error:         true,
			errorContains: text.FORBIDDEN_NODE_TYPE_IN_INCLUDABLE_CHUNK_IMPORTED_BY_PREINIT,
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
			expectedStaticCheckErrors: []string{text.PERMS_SECTION_SHOULD_BE_AN_OBJECT},
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
							path: ` + preinitFile1Path + `
							pattern: %str
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               Path(preinitFile1Path),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(preinitFile1Path)),
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
							path: ` + preinitFile1Path + `
							pattern: %str
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               Path(preinitFile1Path),
					Parsed:             String("a"),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(preinitFile1Path)),
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
							path: ` + nonExistingFilePath + `
							pattern: %str
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               Path(nonExistingFilePath),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(nonExistingFilePath)),
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
					pattern p = str("1"+)
				}
				manifest {
					preinit-files: {
						F: {
							path: ` + preinitFile2Path + `
							pattern: %p
						}
					}
				}
			`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F",
					Path:               Path(preinitFile1Path),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(preinitFile2Path)),
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
							path: ` + preinitFile1Path + `
							pattern: %str
						}
						F2: {
							path: ` + preinitFile2Path + `
							pattern: %str
						}
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimits:      []Limit{minLimitA, minLimitB, threadLimit},
			expectedPreinitFileConfigs: PreinitFiles{
				{
					Name:               "F1",
					Path:               Path(preinitFile1Path),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(preinitFile1Path)),
				},
				{
					Name:               "F2",
					Path:               Path(preinitFile2Path),
					Pattern:            STR_PATTERN,
					RequiredPermission: CreateFsReadPerm(Path(preinitFile2Path)),
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
			expectedStaticCheckErrors: []string{text.PREINIT_FILES_SECTION_SHOULD_BE_AN_OBJECT},
		},
		//Check that the parameters section is forbidden in most modules.

		{
			name:       "the parameters section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
				manifest {
					parameters: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//Check that the databases section is forbidden in most modules.

		{
			name:       "the databases section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
				manifest {
					databases: {}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("databases", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//Check that the parameters section is forbidden in most modules.

		{
			name:       "the parameters section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						parameters: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("parameters", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//Check that the invocation section is forbidden in most modules.

		{
			name:       "the invocation section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						invocation: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("invocation", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//Check that the preinit-files section is forbidden in most modules.

		{
			name:       "the preinit-files section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
					manifest {
						preinit-files: {}
					}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("preinit-files", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//Check that the env section is forbidden in most modules.

		{
			name:       "the env section is not allowed in spec modules",
			moduleKind: SpecModule,
			module: `
						manifest {
							env: {}
						}`,
			error:                     true,
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", SpecModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", UserLThreadModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", TestSuiteModule)},
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
			expectedStaticCheckErrors: []string{text.FmtTheXSectionIsNotAllowedForTheCurrentModuleKind("env", TestCaseModule)},
			expectedLimits:            []Limit{},
		},

		//TODO: improve tests.
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "read any global" {
				testCase.expectedPermissions =
					append(testCase.expectedPermissions, GlobalVarPermission{permbase.Use, "*"}, GlobalVarPermission{permbase.Create, "*"})
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

			tempDir := t.TempDir()
			mainModulePath := filepath.Join(tempDir, MAIN_MODULE_NAME)

			var parentState *GlobalState

			if testCase.parentModule != "" {
				chunk := parse.MustParseChunk(testCase.parentModule)

				srcFile := sourcecode.File{
					NameString:  testCase.parentModuleAbsPath,
					Resource:    testCase.parentModuleAbsPath,
					ResourceDir: filepath.Dir(testCase.parentModuleAbsPath),
					CodeString:  testCase.parentModule,
				}

				parsedChunk := parse.NewParsedChunkSource(chunk, srcFile)
				mod := WrapLowerModule(&inoxmod.Module{
					MainChunk:        parsedChunk,
					TopLevelNode:     parsedChunk.Node,
					ManifestTemplate: chunk.Manifest,
				})

				parentState = NewGlobalState(NewContext(ContextConfig{
					DoNotSpawnDoneGoroutine: true,
				}))
				defer parentState.Ctx.CancelGracefully()
				parentState.Module = mod
				parentState.MainState = parentState

				start := time.Now()
				manifest, _, _, err := mod.PreInit(PreinitArgs{
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

			mod := WrapLowerModule(&inoxmod.Module{
				MainChunk: parse.NewParsedChunkSource(chunk,
					sourcecode.File{
						NameString:             mainModulePath,
						UserFriendlyNameString: mainModulePath,
						Resource:               mainModulePath,
						ResourceDir:            filepath.Dir(mainModulePath),
						IsResourceURL:          false,
						CodeString:             testCase.module,
					},
				),
				TopLevelNode:          chunk,
				Kind:                  testCase.moduleKind,
				ManifestTemplate:      chunk.Manifest,
				InclusionStatementMap: map[*ast.InclusionImportStatement]*IncludedChunk{},
				IncludedChunkMap:      map[string]*IncludedChunk{},
			})

			{
				ctx := NewContext(ContextConfig{
					Permissions: []Permission{
						FilesystemPermission{Kind_: permbase.Read, Entity: PathPattern("/...")},
					},
					DoNotSpawnDoneGoroutine: true,
				})
				ParseLocalIncludedFiles(ctx, IncludedFilesParsingConfig{
					Module:                              mod.Lower(),
					RecoverFromNonExistingIncludedFiles: false,
				})
				ctx.CancelGracefully()
			}

			start := time.Now()
			manifest, _, staticCheckErrors, err := mod.PreInit(PreinitArgs{
				GlobalConsts:          chunk.GlobalConstantDeclarations,
				PreinitStatement:      chunk.Preinit,
				RunningState:          nil,
				ParentState:           parentState,
				AddDefaultPermissions: true,
				AdditionalGlobals:     testCase.additionalGlobals,
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

			}
		})
	}

}
