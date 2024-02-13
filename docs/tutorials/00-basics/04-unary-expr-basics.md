# Unary Expressions 

```
manifest {}

# A number negation is always parenthesized. Integers and floats that are immediately preceded 
# by a '-' sign are parsed as literals.

int = -1     # integer literal
float = -1.0 # float literal

(- int)     # integer negation: 1
(- float)   # float negation: 1.0
(- 1.0)     # float negation

# Boolean negation

!true # false

myvar = true
!myvar # false
```