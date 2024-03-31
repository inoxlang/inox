[Table of contents](./README.md)

---

## Concatenation Operations

Concatenation of strings, byte slices and tuples is performed with a
concatenation expression.

```
concat "a" "b"
# result: "ab"

list = ["b", "c"]
concat "a" ...list
# result: "abc"

concat 0x[00] 0x[11 22]
# result: 0x[00 11 22]

concat #[1] #[2]
# result: #[1, 2]
```

**Parenthesized** concatenation expressions can span several lines:

```
(concat 
    "start"
    "1" # comment
    "2"
    "end"
)
```

[Back to top](#concatenation-operations)
