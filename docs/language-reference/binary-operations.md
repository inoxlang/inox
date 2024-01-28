[Table of contents](./language.md)

---

# Binary Operations

Binary operations are always parenthesized: `(1 + 2)`, `(1 < 2)`, `(a or b)`.\
Parentheses can be omitted around operands of **or**/**and** chains:

```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok
```

- [Arithmetic](#arithmetic)
  - [Addition](#addition)
  - [Substraction](#substraction)
  - [Multiplication](#multiplication)
  - [Division](#division)

- [Ordered Comparison](#ordered-comparison)
- [Equality](#equality)

---

## Arithmetic

### Addition

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be added together.
  - A float can be converted to an integer using `toint`: `(1 + toint(1.0))`,
    **the function panics if there is precision loss**.
  - An integer can be converted to a float using `tofloat`: `(1.5 + tofloat(1))`

- Durations can be added to datetimes, the result is a datetime: `(now() + 1h)`,
  `(1h + now())`
- Durations can be added together: `(1h + 1s)`

### Substraction

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be added together (details in [addition](#addition)).
- Durations can be substraction from one another: `(1h + 1s)`

### Multiplication

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be added together (details in [addition](#addition)).
- Integer multiplication produces an integer

### Division

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be added together (details in [addition](#addition)).
- Integer division produces an integer

---

## Ordered Comparison

The `<, <=, >=, >` comparisons are supported by all **comparable** Inox types.
Values of different types generally cannot be compared.

```
(1 < 2)
(1ms < 1s)
(ago(1h) < now())
("a" < "b")
```

**Comparable Inox types**: int, float, byte, string, byte-count, line-count,
rune-count, year, date, datetime, duration, frequency, byte-rate, ulid, port.

The ordering of strings is the
[natural sort order](https://en.wikipedia.org/wiki/Natural_sort_order).

---

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

Addition

- integer comparison: `(1 < 2)`
- floating point addition: `(1.0 + 2.5)`
- floating point comparison: `(1.0 < 2.5)`
- deep equality: `({a: 1} == {a: 1})`
- logical operations: `(a or b)`, `(a and b)`
- exclusive-end range: `(0 ..< 2)`
- inclusive-end range: `(0 .. 2)`

---

## Equality

- [Objects & Records](#objects--records)
- [Lists, Key Lists & Tuples](#lists-key-lists--tuples)
- [String-Like Values](#string-like-values)
- [Bytes-Like Values](#bytes-like-values)
- [Booleans](#booleans)
- [Numbers, Quantities and Rates](#numbers-quantities--rates)
- [Ranges](#ranges)
- [Secrets](#secrets)
- [Go Functions](#go-functions)
- [Inox Functions](#inox-functions)
- [Mapping](#mappings)
- [Paths](#paths)
- [Scheme](#schemes)
- [Hosts](#hosts)
- [URLs](#urls)
- [Ports](#ports)
- [Time Types](#time-types)
- [Errors](#errors)
- [Regex Patterns](#regex-patterns)
- [Avanced String Patterns](#advanced-string-patterns)
- [Object Pattern](#object-patterns)
- [Record Pattern](#record-patterns)

All values can be compared using the `==` operator. Equality definitions are not
definitive but in general:
- Simple values (integers, floats, strings, paths, URLs, ...) are equal if they have the same type and the exact same number or string value.
- More complex Inox values such as objects and lists are compared using structural equality if they have the same type. 

### Objects & Records

- Two **objects** are equal if they have the same property names and their
  properties are equal; `({a: 1} == {a: 1}) # true`. Meta properties (e.g.
  `_url_`) are not taken into account.

- Two **records** are equal if they have the same property names and their
  properties are equal. `(#{a: 1} == #{a: 1}) # true`

- Objects and records are never equal but this could change.

### Lists, Key Lists & Tuples

- Two **lists** are equal if they have the same number of elements and their
  elements are equal: `([1, 2] == [1, 2]) # true`

- Two **keylists** are equal if they have the same number of elements and their
  elements are equal: `(.{name, age} == .{name, age})` # true

- Two **tuples** are equal if they have the same number of elements and their
  elements are equal: `(#[1, 2] == #[1, 2]) # true`

- Lists and tuples are never equal but this could change.

### String-Like Values

String-like values (e.g strings, string concatenations) are equal if they
resolve to the same string. Note that paths and URLs are similar to strings but
are **not string-like**.

### Bytes-Like Values

Bytes-like values (e.g byte slices, bytes concatenations) are equal if they
resolve to the same byte slice.

### Booleans

- `true` is only equal to its own value (no coercion)
- `false` is only equal to its own value (no coercion)

### Numbers, Quantities & Rates

Numbers, quantities and rates of different types are **not equal**. Numbers,
quantities and rates of the same type are equal if they have the same value.

All the following comparisons return `false`:

```
(1.0 == 1)
(1ln == 1)
(1rn == 1)
(1B/s == 1)
```

### Ranges

Ranges of different types are **not equal**.

All the following comparisons return `false`:

```
((1.0 .. 2.0) == (1 .. 2))
((1B .. 2B) == (1 .. 2))
('A'..'B' == 65..66)
```

Ranges **without a defined start** (lower bound) are never equal to ranges with
a defined start.

All the following comparisons return `false`:

```
(..1 == 0..1)
(..1.0 == 0.0..1.0)
(..1B == 0B..1B)
```

### Secrets

**The comparison of two secrets always returns false.**

### Go Functions

Go functions are compared by evaluating
`(reflect.ValueOf(func1) == reflect.ValueOf(func2))`. Two different Go closures
are never equal. Go methods are never equal, even if they are bound to the same
Go object. A Go function, closure or method is obviously equal to itself.

```
# true
print((print == print))

l = [];

# false
print((l.append == l.append))

method = l.append

# true
print((method == method))
```

### Inox Functions

Two Inox functions are equal by identity.

Note that two functions that are defined in the same file, but are from two
different modules, are not the same. This may change in the future.

### Mappings

Mappings are equal by identity.

### Paths

Two paths are equal if they are exactly the same: `/a/b` is not equal to `/a//b`
or `/a/b/`.

### Scheme

Two schemes are equal if they are exactly the same.

### Hosts

Two hosts are equal if they are exactly the same:

- `http://example.com` is not equal to `https://example.com:80`
- `https://example.com` is not equal to `https://example.com:443`

**This may change in the future.**

### URLs

Two URLs are equal if they are exactly the same:

- `https://example.com/a/b` is not equal to `https://example.com/a//b` or
  `https://example.com/a/b/`.
- `https://example.com/a/b` is not equal to `https://example.com/a/b?q=search`
- `https://example.com/a/b` is not equal to `https://example.com/a/b#fragment`

**This may change in the future.**

### Ports

Two ports are equal if they have the same number, so `:8080 == :8080/http == :8080/https`.

**This may change in the future.**

### Time Types

Comparing values of different time types always returns `false`:
```
(2020y-UTC == 2020y-1mt-1d-UTC) # year and date
(2020y-UTC == 2020y-1mt-1d-1h-UTC) # year and datetime
(2020y-1mt-1d-UTC == 2020y-1mt-1d-1h-UTC) # date and datetime
```

Values of a given time type are equal if they represent the same time instant.
Two times can be equal even if they are in different locations. 

### Errors

WIP

### Regex Patterns

Two regex patterns are equal if they have exactly the same source expression,
therefore the following comparisons are false:
```
(%`aa*` == %`a+`)
(%`(\d+)` == %`\d+`)
```

## Advanced String Patterns



### Object Patterns

Two **object patterns** are equal if they have the same inexactness, the same
property names and their entry patterns are equal. Two entry patterns are equal
if they have the same optionality and dependencies.

| pattern A    | pattern B           | equality |
| ------------ | ------------------- | -------- |
| `%{a: int}`  | `%{a: int}`         | true     |
| `%{a?: int}` | `%{a: int}`         | false    |
| `%{a: int}`  | `%{a: int, c: int}` | false    |

### Record Patterns

Two **record patterns** are equal if they have the same inexactness, the same
property names and their entry patterns are equal. Two entry patterns are equal
if they have the same optionality.

| pattern A          | pattern B                 | equality |
| ------------------ | ------------------------- | -------- |
| `%record{a: int}`  | `%record{a: int}`         | true     |
| `%record{a?: int}` | `%record{a: int}`         | false    |
| `%record{a: int}`  | `%record{a: int, c: int}` | false    |

[Back to top](#binary-operations)
