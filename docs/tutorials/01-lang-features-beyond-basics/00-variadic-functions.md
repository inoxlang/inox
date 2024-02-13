# Variadic Functions 

```
manifest {}

# Variadic functions are functions whose last parameter can aggregate any number of arguments. 
# This is indicated by the '...' syntax. This parameter is named the variadic parameter and is always of type Array.

fn return_variadic_arguments(first, ...rest){
    return rest
}

return_variadic_arguments(1)        # empty array 
return_variadic_arguments(1, 2)     # Array(2)
return_variadic_arguments(1, 2, 3)  # Array(2, 3)

# The variadic parameter can have a type annotation:

fn sum(...integers int){
    i = 0
    for int in integers {
        i += int
    }
    return i
}

print(sum())
print(sum(1))
print(sum(1, 2))

# ...[2, 3] is a spread argument
print(sum(1, ...[2, 3]))
```