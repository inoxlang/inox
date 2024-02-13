# List Patterns 

```
manifest {}

# List pattern matching any list containing (only) integers.
pattern int_list = []int

print `([] match []int):` ([] match []int)
print `([1] match []int):` ([1] match []int)
print `([1, "a"] match []int):` ([1, "a"] match []int)

# List pattern matching any list of length 2 having an integer as first element 
# and a string-like value as second element.
pattern pair = [int, str]

print `\n([1, "a"] match [int, str]):` ([1, "a"] match [int, str])
```