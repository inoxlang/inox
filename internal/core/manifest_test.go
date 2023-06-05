package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

	type testCase struct {
		name                      string
		inputModule               string
		expectedPermissions       []Permission
		expectedLimitations       []Limitation
		expectedResolutions       map[Host]Value
		expectedDatabaseConfigs   DatabaseConfigs
		error                     bool
		expectedStaticCheckErrors []string
	}

	var testCases = []testCase{

		{
			name: "host resolution",
			inputModule: `
				manifest { 
					host_resolution: :{
						ldb://main : /mydb
					}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: map[Host]Value{"ldb://main": Path("/mydb")},
			error:               false,
		},
		{
			name:                "empty manifest",
			inputModule:         `manifest {}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "read_any_global",
			inputModule: `manifest { 
					permissions: { read: {globals: "*"} }
				}`,
			expectedPermissions: []Permission{GlobalVarPermission{permkind.Read, "*"}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "create_routine",
			inputModule: `manifest { 
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
			inputModule: `manifest { 
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
			inputModule: `
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
			inputModule: `manifest { 
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
			inputModule: `manifest { 
					permissions: {
						read: mem://a.com
					}
				}`,
			error: true,
		},
		{
			name: "host_pattern_with_unsupported_scheme",
			inputModule: `manifest { 
					permissions: { read: %ws://*.com }
				}`,
			error: true,
		},
		{
			name: "dns",
			inputModule: `manifest { 
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
			inputModule: `manifest { 
					permissions: {
						read: {
							dns: %https://**.com
						}
					}
				}`,
			error: true,
		},
		{
			name: "see email addresses",
			inputModule: `manifest { 
					permissions: {
						see: { values: %emailaddr }
					}
				}`,
			error: true,
		},
		{
			name: "see email addresses & ints",
			inputModule: `manifest { 
				permissions: {
					see: { values: [%emailaddr, %int] }
				}
			}`,
			error: true,
		},
		{
			name: "invalid node type in preinit block",
			inputModule: `
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
			inputModule: `
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
			inputModule: `manifest { 
					preinit-files: {}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "empty_databases",
			inputModule: `manifest { 
					databases: {}
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               false,
		},
		{
			name: "correct_databases",
			inputModule: `manifest { 
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
			name: "database_with_invalid_resource",
			inputModule: `manifest { 
					databases: {
						main: {
							resource: 1
						}
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_RESOURCE_SHOULD_BE_HOST_OR_URL},
		},
		{
			name: "database_with_invalid_resolution_data",
			inputModule: `manifest { 
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
			name: "database_description_should_be_an_object",
			inputModule: `manifest { 
					databases: {
						main: 1
					}
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES__DB_CONFIG_SHOULD_BE_AN_OBJECT},
		},
		{
			name: "databases_section_should_be_an_object",
			inputModule: `manifest { 
					databases: 1
				}`,
			error:                     true,
			expectedStaticCheckErrors: []string{DATABASES_SECTION_SHOULD_BE_AN_OBJECT},
		},
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

			chunk := parse.MustParseChunk(testCase.inputModule)

			mod := &Module{
				MainChunk: parse.NewParsedChunk(chunk, parse.InMemorySource{
					NameString: "test",
					CodeString: testCase.inputModule,
				}),
				ManifestTemplate: chunk.Manifest,
			}

			manifest, _, staticCheckErrors, err := mod.PreInit(PreinitArgs{
				GlobalConsts:          chunk.GlobalConstantDeclarations,
				Preinit:               chunk.Preinit,
				RunningState:          nil,
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
			}
		})
	}

}
