[Table of contents](./README.md)

---

# Transient Types

A **transient** value is a value that cannot be persisted in the database (not
serializable). Transient values are generally more memory-efficient and have
better access performance.

## Memory Allocation

**(Work in progress)**

Memory allocation logic will depend on the expected lifespan of the module
performing the allocation. For example HTTP handler modules often finish in less
than 1 second, so specific optimizations could be implemented.

## Structs

⚠️ This feature is not fully implemented yet and is subject to change.

Structs are transient values that only exist in the stack on in the module's
heap. Accessing the field of a struct is faster than accessing a
property/element of other Inox types (e.g. objects).

```
struct Position2D {
    x int
    y int
}

struct Lexer {
    index int
    input string
    tokens [.]Token

    fn next() (Token, bool) {
        ....
    }
}

struct Token {
    type TokenType
    value string
}
```

Struct methods will be executed by the [low level VM](https://github.com/inoxlang/inox/issues/32).

## Arrays

⚠️ This feature is not implemented yet and is subject to change.

```
integers = new([.]int, 100)
```

## Slices

⚠️ This feature is not implemented yet and is subject to change.

```
slice = new([:]int, 100)

sub_slice = slice[0:10]
```

[Back to top](#transient-types)
