[Table of contents](./language.md)

---

# Binary Operations

Binary operations are always parenthesized:

- integer addition: `(1 + 2)`
- integer comparison: `(1 < 2)`
- floating point addition: `(1.0 + 2.5)`
- floating point comparison: `(1.0 < 2.5)`
- deep equality: `({a: 1} == {a: 1})`
- logical operations: `(a or b)`, `(a and b)`
- exclusive-end range: `(0 ..< 2)`
- inclusive-end range: `(0 .. 2)`

ℹ️ Parentheses can be omitted around operands of **or**/**and** chains:

```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok
```

This [script](../examples/basic/binary-expressions.ix) contains most possible
binary operations.

## Match

The binary **match** operation returns true if the value on the left matches the
pattern on the right. The pattern does not require a `%` prefix, unless it's a
pattern literal.

```
object = {a: 1}

(object match {a: 1})  # the right operand is NOT an object here, it's a pattern.
(object match %{a: 1}) # equivalent to the previous line

(object match {a: int})
(object match %{a: int}) # equivalent
```

## Comparison

The `<, <=, >=, >` comparisons are supported by all **comparable** Inox types.
Values of different types generally cannot be compared.

```
(1 < 2)
(1ms < 1s)
(ago(1h) < now())
```

**Comparable Inox types**: int, float, byte, string, byte-count, line-count, rune-count,
year, date, datetime, duration, frequency, byte-rate, port.

[Back to top](#)
