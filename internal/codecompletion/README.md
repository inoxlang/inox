# Code Completion

This package implements completion for Inoxlang code. The main API is the `FindCompletions` function and the `Completion` type.
There are several completion modes: 
- ShellCompletions - used by the REPL 
- LspCompletions - used by the project (LSP) server