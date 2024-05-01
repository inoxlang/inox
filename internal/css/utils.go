package css

import "strings"

func DoesClassListStartWithUppercaseLetter(classList string) bool {
	names := strings.Split(classList, " ")
	if len(names) == 0 {
		return false
	}
	firstName := strings.TrimSpace(names[0])
	return firstName != "" && firstName[0] >= 'A' && firstName[0] <= 'Z'
}
