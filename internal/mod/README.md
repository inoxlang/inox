## Module Execution

First the risk score is computed, if it's too high a prompt is shown to the user to confirm the execution.

If the bytecode interpretration is chosen the module is compiled & executed in the bytecode interpreter (VM).
Otherwise the tree walk interpreter executes the module. If debugging is enabled the debugger is attached 
to the global state just before the execution starts.
