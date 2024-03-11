package css

import "strings"

// Example: "--primary-bg"
type VarName string

func HasValidVarNamePrefix(name string) bool {
	return strings.HasPrefix(name, "--")
}
