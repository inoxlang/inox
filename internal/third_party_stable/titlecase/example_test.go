package titlecase

import "fmt"

func Example() {

	fmt.Println(Title("this and that"))
	fmt.Println(Title("TURN OF CAPS LOCK"))

	// Output:
	// This and That
	// Turn of Caps Lock
}
