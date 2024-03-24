[Table of contents](./README.md)

---

# Unary Operations

- [Number negation](#number-negation)
- [Boolean negation](#boolean-negation)
- [Boolean conversion](#boolean-conversion)

## Number Negation

```
- int     # integer negation: -1
- float   # float negation: -1.0
- 1.0     # float negation
- 1       # integer negation
```

`-1` and `-1.0` are literals because there is no space between the minus sign
and the first digit.

## Boolean Negation

```
!true # false

myvar = true
!myvar # false
```

[Back to top](#unary-operations)

## Boolean Conversion

The boolean conversion operation coerces its operand to a boolean.

```
value = []
value? # false

value = [1]
value? # true
```

- Empty indexables and containers are considered **falsy**.
- `nil` and `false` are **falsy**.
- Zero is **falsy**, regardless of its type (integer, float, quantity or rate)
- An iterable that is neither an `indexable` nor a `container` is considered
  **truthy**, even if it has no elements. The conversion operation does not
  iterate over because doing so might 'consume it'.
- All other Inox values are **truthy**.

**Examples**

| value    | result  |
| -------- | ------- |
| `nil`    | `false` |
| `[1]`    | `true`  |
| `[]`     | `false` |
| `"a"`    | `true`  |
| `""`     | `false` |
| `{a: 1}` | `true`  |
| `{}`     | `false` |
| `1`      | `true`  |
| `0`      | `false` |
| `1.0`    | `true`  |
| `0.0`    | `false` |
| `1kB`    | `true`  |
| `0B`     | `false` |
| `1kB/s`  | `true`  |
| `0B/s`   | `false` |
| `true`   | `true`  |
| `false`  | `false` |

### Coercion table

| value                                 | result  |
| ------------------------------------- | ------- |
| `nil`                                 | `false` |
| `indexable with at least one element` | `true`  |
| `empty indexable`                     | `false` |
| `container with at least one element` | `true`  |
| `empty container`                     | `false` |
| `non-zero integral`                   | `true`  |
| `0 (any integral type)`               | `false` |
| `non-zero float`                      | `true`  |
| `0.0 and -0.0`                        | `false` |
| `non-zero quantity`                   | `true`  |
| `0 (any quantity type)`               | `false` |
| `non-zero rate`                       | `true`  |
| `0 (any rate type)`                   | `false` |
| `true`                                | `true`  |
| `false`                               | `false` |
| **other types**                       | `true`  |
