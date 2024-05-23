package globalnames

const (
	CURRENT_TEST = "__test"
)

var (
	//globals that are not inherited by test suites and test cases from their parent state.
	TEST_ITEM_NON_INHERITED_GLOBALS = []string{
		CURRENT_TEST,
		PREINIT_DATA,
		PROJECT_SECRETS,
	}
)
