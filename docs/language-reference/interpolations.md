[Table of contents](./README.md)

---

# Interpolations

## Regular Strings

```
`Hello ${name}`

`Hello ! 
I am ${name}`
```

## Checked Strings

In Inox checked strings are strings that are validated against a pattern. When
you dynamically create a checked string all the interpolations must be
explicitly typed:

```
pattern integer = %`(0|[1-9]+[0-9]*)`

pnamespace math. = {
    expr: %str( integer (| "+" | "-") integer)
    int: %integer
}

one = "1"
two = "2"

checked_string = %math.expr`${int:one}+${int:two}`
```

## URL Expressions

When you dynamically create URLs the interpolations are restricted based on
their location (path, query).

```
https://example.com/api/{path}/?x={x}
```

- interpolations before the **'?'** are **path** interpolations
  - the strings/characters **..** | **\*** | **\\** | **?** | **#** are
    forbidden
  - **':'** is forbidden at the start of the finalized path (after all
    interpolations have been evaluated)
- interpolations after the **'?'** are **query** interpolations
  - the characters **'&'** and **'#'** are forbidden

URL path interpolations:

```
path = /index.html        # you can also use the string "/index.html"
https://example.com{path} 
# result: https://example.com/index.html

path = /users/1           # you can also use the path ./users/1
https://example.com/api/{path} 
# result: https://example.com/api/users/1
```

URL query interpolations:

```
param_value = "x"
https://google.com/?q={param_value}
# result: https://google.com?q=x

param_value = "git"
https://google.com/?q={param_value}hub
# result: https://google.com?q=github
```

Host aliases:

```
@host = https://example.com   # host literal
@host/index.html
```

## Path Expressions

```
path = /.bashrc     # you can also use the path ./.bashrc or a string
/home/user/{path}
# result: /home/user/.bashrc
```

⚠️ Some sequences such as '..' are allowed in the path but not in the
interpolation !

```
# ok
/home/user/dir/..

path = /../../etc/passwd
/home/user/{path}
# error: result of a path interpolation should not contain any of the following substrings: '..', '\', '*', '?'
```

[Back to top](#interpolations)
