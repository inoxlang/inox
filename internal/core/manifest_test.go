package internal

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
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               true,
		},
		{
			name: "host_pattern_with_unsupported_scheme",
			inputModule: `manifest { 
					permissions: { read: %ws://*.com }
				}`,
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               true,
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
			expectedPermissions: []Permission{},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               true,
		},
		{
			name: "see email addresses",
			inputModule: `manifest { 
					permissions: {
						see: { values: %emailaddr }
					}
				}`,
			expectedPermissions: []Permission{ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN}},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               true,
		},
		{
			name: "see email addresses & ints",
			inputModule: `manifest { 
				permissions: {
					see: { values: [%emailaddr, %int] }
				}
			}`,
			expectedPermissions: []Permission{
				ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN},
				ValueVisibilityPermission{Pattern: INT_PATTERN},
			},
			expectedLimitations: []Limitation{},
			expectedResolutions: nil,
			error:               true,
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
			expectedLimitations:       []Limitation{},
			expectedResolutions:       nil,
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
			expectedLimitations:       []Limitation{},
			expectedResolutions:       nil,
			expectedStaticCheckErrors: []string{PERMS_SECTION_SHOULD_BE_AN_OBJECT},
			error:                     true,
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.expectedPermissions, manifest.RequiredPermissions)
				assert.EqualValues(t, testCase.expectedLimitations, manifest.Limitations)
				assert.EqualValues(t, testCase.expectedResolutions, manifest.HostResolutions)
			}

		})
	}

}
