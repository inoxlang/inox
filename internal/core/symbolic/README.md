## Symbolic

This package implements symbolic evaluation and typechecking.

- [eval.go](./eval.go): main evaluation logic
- [eval_call.go](./eval.go): function call evaluation logic
- [state.go](./state.go): symbolic evaluation state
- [data.go](./data.go): symbolic evaluation and type checking data (e.g. typechecking errors, warnings)
- [narrowing_widening.go](./eval.go): type narrowing and widening
- [multivalue.go](./multivalue.go): symbolic representation of value unions
- [intersection.go](./intersection.go): type intersection
- [readonly.go](./readonly.go): interfaces and helpers related to readonly values (unrelated to immutable values)
- [pattern.go](./pattern.go): symbolic representation of Inox patterns (e.g object pattens)
- [error.go](./error.go): error message constants and formatting

The other files contain the symbolic representations of Inox values: most of the files in the `core` package have a counterpart in this package.