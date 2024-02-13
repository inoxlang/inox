# Binary Expressions 

```
manifest {}

# Binary operations are always parenthesized:

int = (1 + 2)
float = (1.0 + (5.0 + 2.0))
range1 = (0 .. 2)   # inclusive end
range2 = (0 ..< 3)  # exclusive end

# Parentheses can be omitted around operands of or/and chains:

a = true; b = false; c = true

(a or b or c)      
(1 < 2 or 2 < 3)

# 'or' and 'and' cannot be mixed in the same chain
# (a or b and c)
```