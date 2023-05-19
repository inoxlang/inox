package internal

import (
	"testing"
	"time"

	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestEvalManifest(t *testing.T) {

	defaultGlobalPermissions := []Permission{
		GlobalVarPermission{permkind.Read, "*"},
		GlobalVarPermission{permkind.Use, "*"},
		GlobalVarPermission{permkind.Create, "*"},
	}
	LimRegistry.RegisterLimitation("a", TotalLimitation, 0)
	LimRegistry.RegisterLimitation("b", ByteRateLimitation, 0)

	testCases := []struct {
		name                string
		inputModule         string
		expectedPermissions []Permission
		expectedLimitations []Limitation
		expectedResolutions map[Host]Value
		error               bool
	}{
		{
			"host resolution", `
			manifest { 
				host_resolution: :{
					ldb://main : /mydb
				}
			}
			`,
			[]Permission{},
			[]Limitation{},
			map[Host]Value{"ldb://main": Path("/mydb")},
			false,
		},
		{
			"empty manifest", `manifest {}`,
			[]Permission{},
			[]Limitation{},
			nil,
			false,
		},
		{
			"read_any_global", `manifest { 
				permissions: { read: {globals: "*"} }
			}`,
			[]Permission{GlobalVarPermission{permkind.Read, "*"}},
			[]Limitation{},
			nil,
			false,
		},
		{
			"create_routine", `manifest { 
				permissions: {
					create: {routines: {}} 
				}
			}`,
			[]Permission{RoutinePermission{permkind.Create}},
			[]Limitation{},
			nil,
			false,
		},
		{
			"create_routine", `manifest { 
				permissions: {
					create: {routines: {}} 
				}
			}`,
			[]Permission{RoutinePermission{permkind.Create}},
			[]Limitation{},
			nil,
			false,
		},
		{
			"read_@const_var", `
				const (
					URL = https://example.com/
				)
				manifest { 
					permissions: { read: $$URL}
				}
			`,
			[]Permission{HttpPermission{permkind.Read, URL("https://example.com/")}},
			[]Limitation{},
			nil,
			false,
		},
		{
			"limitations", `
			manifest { 
				limits: {
					"a": 100ms
				}
			}
			`,
			[]Permission{},
			[]Limitation{
				{Name: "a", Kind: TotalLimitation, Value: int64(100 * time.Millisecond)},
			},
			nil,
			false,
		},

		{
			"host_with_unsupported_scheme", `
			manifest { 
				permissions: {
					read: mem://a.com
				}
			}
			`,
			[]Permission{},
			[]Limitation{},
			nil,
			true,
		},
		{
			"host_pattern_with_unsupported_scheme", `
			manifest { 
				permissions: { read: %ws://*.com }
			}
			`,
			[]Permission{},
			[]Limitation{},
			nil,
			true,
		},
		{
			"dns", `
			manifest { 
				permissions: {
					read: {
						dns: %://**.com
					}
				}
			}
			`,
			[]Permission{
				DNSPermission{permkind.Read, HostPattern("://**.com")},
			},
			[]Limitation{},
			nil,
			false,
		},
		{
			"dns_host_pattern_literal_with_scheme", `
			manifest { 
				permissions: {
					read: {
						dns: %https://**.com
					}
				}
			}
			`,
			[]Permission{},
			[]Limitation{},
			nil,
			true,
		},
		{
			"see email addresses",
			`manifest { 
				permissions: {
					see: { values: %emailaddr }
				}
			}`,
			[]Permission{ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN}},
			[]Limitation{},
			nil,
			true,
		},
		{
			"see email addresses & ints",
			`manifest { 
				permissions: {
					see: { values: [%emailaddr, %int] }
				}
			}`,
			[]Permission{
				ValueVisibilityPermission{Pattern: EMAIL_ADDR_PATTERN},
				ValueVisibilityPermission{Pattern: INT_PATTERN},
			},
			[]Limitation{},
			nil,
			true,
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

			manifest, err := mod.EvalManifest(ManifestEvaluationConfig{
				GlobalConsts:          chunk.GlobalConstantDeclarations,
				RunningState:          nil,
				AddDefaultPermissions: true,
			})

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
