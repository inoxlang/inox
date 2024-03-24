[Table of contents](./README.md)

---

# Binary Operations

Operands of binary operations need to be parenthesized, unless they are 'simple':

```
n = 1 + 2         # ok

n = 1 + 2 + 3     # error
n = (1 + 2) + 3   # ok
n = 1 + (2 + 3)   # ok
```

Parentheses can be omitted around operands of parenthesized **or**/**and** chains:

```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok
```

⚠️ In some places binary operations need to be parenthesized:

```
1 + 2           # (statement) error
(1 + 2)         # (statement) ok

print 1 + 2     # (function call) valid but not equivalent to `print (1 + 2)`
print (1 + 2)   # ok


name_or_nil = ...
concat "Hello " name_or_nil ?? b    # error
concat "Hello " (name_or_nil ?? b)  # ok
```

- [Arithmetic](#arithmetic)
  - [Addition](#addition)
  - [Substraction](#substraction)
  - [Multiplication](#multiplication)
  - [Division](#division)
- [Logical Operations](#logical-operations)
- [Substrof](#substrof)
- [Keyof](#keyof)
- [Range Operators](#range-operators)
- [In](#in)
- [Nil Coalescing Operator](#nil-coaelescing-operator)
- [Pattern Difference](#pattern-difference)
- [Match](#match)
- [Ordered Comparison](#ordered-comparison)
- [Equality](#equality)
- [Is](#is)

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
  be used in a substraction (details in [addition](#addition)).
- Durations can be substracted from one another: `(1h + 1s)`

### Multiplication

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be used in a multiplication (details in [addition](#addition)).
- Integer multiplication produces an integer

### Division

- Integers and floats cannot be mixed, they have to be explicitly converted to
  be used in a division (details in [addition](#addition)).
- Integer division produces an integer

---

## Logical Operations

`or` and `and` are binary logical operators, however parentheses can be omitted
around operands of **or**/**and** chains:

```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok
```

---

# Substrof

The `substrof` operator is defined for string-like and bytes-like values, the
two interfaces can be mixed.

```
("A" substrof "AB")
(0d[65] substrof "AB")
("A" substrof 0d[65 66])
(0d[65] substrof 0d[65 66])
```

---

# Keyof

The `keyof` operator returns true if the left operand (string-like) is the name
of a property in the object on the right.

```
("a" keyof {a: 1}) # true
```

---

## Range Operators

The `..` and `..<` range operators create a range. The left operand (lower
bound) must be smaller than the right operand (upper bound).

`..<` is named the **exclusive end range operator**.

```
# integer range
(1 .. 3) # 1 to 3 int range
(1 ..< 3) # 1 to 2 int range

# float range
(1.0 .. 3.0) # 1.0 to 3.0 float range
(1.0 ..< 3.0) # 1.0 to 3.0 float range (exclusive end)

# quantity range
(1B .. 3B) # 1B to 3B quantity range
```

---

## In

The `in` operator returns true if the left operand is an element of the
**container** on the right.

The following binary operations evaluate to `true`.

```
(1 in [1, 2, 3])
(1 in {a: 1})
(1 in 1..2)
```

The `not-in` operator is the opposite of `in`.

---

## Nil Coaelescing Operator

The `??` operator returns the right operand if the left operand is `nil`.

```
(nil ?? 1) # 1
(2 ?? 1) # 2
```

---

## Pattern Difference

The pattern difference operator (`\`) creates a new pattern that matches all
values matched by the left operand, except for those matched by the right
operand.

```
pattern p = (%int \ %int(1..2))

# false
(1 match p)

# true
(0 match p)

pattern p = (%int \ 1)

# false
(1 match p)

# true
(0 match p)
```

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

The `not-match` operator is the opposite of `match`.

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

- Simple values (integers, floats, strings, paths, URLs, ...) are equal if they
  have the same type and the exact same number or string value.
- More complex Inox values such as objects and lists are compared using
  structural equality if they have the same type.

The `!=` operator is the opposite of `==`.

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

Two ports are equal if they have the same number, so
`:8080 == :8080/http == :8080/https`.

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

```text
(%`aa*` == %`a+`)
(%`(d+)` == %`d+`)
```

## Advanced String Patterns

Two advanced string patterns are equal if they have the same regex or the exact
same structures and sub patterns.

**This may change in the future.**

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

---

### Is

The `is` operator returns true if the two operands have the same identity.

The `is-not` operator is the opposite of `is`.

(work in progress)

[Back to top](#binary-operations)
