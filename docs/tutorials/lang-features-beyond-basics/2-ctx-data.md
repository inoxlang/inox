# Context Data 

```
manifest {
    permissions: {
        create: {threads: {}}
    }
}

# Each Inox module instance has a context that can, among other things, store data. Context data entries can be defined once 
# and the value should fall in one of the following categories: sharable (e.g. objects, sets), immutable (e.g. integers, records), 
# clonable (e.g. lists).

# Add the context data entry /x with 1 as value.
add_ctx_data(/x, 1)

# Retrieve the value of the entry /x.
value = ctx_data(/x)

# Retrieving the value of an undefined entry returns nil.
print("undefined entry /y:", ctx_data(/y))

# Retrieve the value of the entry /x and check that the value is an integer.
value = ctx_data(/x,  %int)

# Child modules have access to the context data of their parent and can override the entries.
lthread = go do {
    print("/x from parent:", ctx_data(/x))

    add_ctx_data(/x, 2)

    print("overriden /x:", ctx_data(/x))
}

lthread.wait_result!()

# Learn more about Inox contexts here: https://github.com/inoxlang/inox/blob/main/docs/language-reference/context.md.
```