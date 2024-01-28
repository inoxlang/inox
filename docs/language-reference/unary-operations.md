[Table of contents](./language.md)

---

# Unary Operations

## Number Negation

A number negation is always parenthesized. Integers and floats that are
immediately preceded by a '-' sign are parsed as literals.

```
# -1 and -1.0 are literals because there is no space between the minus sign and the first digit.
int = -1        
float = -1.0

(- int)     # integer negation: 1
(- float)   # float negation: 1.0
(- 1.0)     # float negation
(- 1)       # integer negation
```

## Boolean Negation

```
!true # false

myvar = true
!myvar # false
```

[Back to top](#unary-operations)
