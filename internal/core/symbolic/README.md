## Symbolic

### Code Organization

This package implements symbolic evaluation and typechecking.
**This package does not implement a type system in the traditional sense.**

- [eval.go](./eval.go): main evaluation logic
- [eval_call.go](./eval.go): function call evaluation logic
- [state.go](./state.go): symbolic evaluation state
- [data.go](./data.go): symbolic evaluation and type checking data (e.g. typechecking errors, warnings)
- [narrowing_widening.go](./narrowing_widening.go): type narrowing and widening
- [multivalue.go](./multivalue.go): symbolic representation of value unions
- [intersection.go](./intersection.go): type intersection
- [readonly.go](./readonly.go): interfaces and helpers related to readonly values (unrelated to immutable values)
- [pattern.go](./pattern.go): symbolic representation of Inox patterns (e.g object pattens)
- [error.go](./error.go): error message constants and formatting

The other files contain the symbolic representations of Inox values: most of the files in the `core` package have a counterpart in this package.

### Implementation Details

**Representation**

Symbolic values are immutable. They are replaced when a mutation happens in analyzed code.
In the following example the symbolic representation of {a: 1} (an *Object) is immutable. When `obj` is mutated the scope is updated
with a new *Object of value {a: 2} assigned to `obj`.

```
obj = {a: 1}
obj.a = 2   
```
