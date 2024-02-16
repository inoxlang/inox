[Table of contents](./README.md)

---

# Patterns

- [Named patterns](#named-patterns)
- [Object patterns](#object-patterns)
- [List patterns](#list-patterns)
- [String patterns](#string-patterns)
- [Union Patterns](#union-patterns)
- [Pattern namespaces](#pattern-namespaces)
- [Path Patterns](#path-patterns)
- [Host and URL Patterns](#host-and-url-patterns)
- [Function Patterns](#function-patterns)

In Inox a pattern is a **runtime value** that matches values of a given kind and
shape.\
Besides the pattern [literals](./literals.md), there are other kinds of patterns in
Inox such as object patterns `%{a: int}`.\
Even though patterns are created at run time, they can act as types:

```
pattern small_int = int(0..10)

# small_int is created at run time but it can be used in type annotations:
var n small_int = 0
```

ℹ️ In summary you will mostly define **patterns**, not types.

## Named Patterns

Named patterns are equivalent to variables but for patterns, there are many
built-in named patterns such as: `int, str, bool`. Pattern definitions allow you
to declare a pattern.

```
pattern int-list = []int

# true
([1, 2, 3] match int-list) 

pattern user = {
    name: str
    friends: []str
}
```

⚠️ Named patterns cannot be reassigned and their values cannot reference other
named patterns that are not yet defined.

Some named patterns are callable. For example if you want a pattern that matches
all integers in the range 0..10, you can do the following:

```
pattern zero-to-ten = int(0..10)
```

Creating a named pattern `%user` does not prevent you to name a variable `user`:

```
pattern user = {
    name: str
}

user = {name: "foo"}

# true
(user match user)

# alternative syntax
(user match %user)

# assign %user to a variable
my_pattern = %user

(user match $my_pattern)
```

<details>

<summary>Forbidden definition locations</summary>

Patterns can only be defined at the top level before any function declaration, and before any reference to a function declared further below.
```
# ok
pattern a = {a: 1}

fn f(){}

# not allowed: the definition is after a function declaration.
pattern b = {a: 1}
```

```
f()

# not allowed: the definition is after a call to a function that is declared further below.
pattern a = {a: 1}

fn f(){}
```

</details>

## Object Patterns

```
# object pattern with a single property
%{
    name: str
}

# same pattern stored in a named pattern ('%' not required)
pattern object_pattern = {
    name: str
}

# true
({name: "John"} match object_pattern) 

pattern other_pattern = {
    name: str
    account: {  # '%' not required here 
        creation-date: date
    }
}
```

⚠️ By default object patterns are **inexact**: they accept additional properties.

```
# true
({name: "John"} match {}) 

pattern user = {
    name: str
}

# true
({name: "John", additional_prop: 0} match user)
```

## List Patterns

The syntax for patterns that match a list with **elements of the same type**
(only integers, only strings, etc.) is as follows:

```
pattern int-list = []int

([] match pattern) # true
([1] match pattern) # true
([1, "a"] match pattern) # false
```

<details>

**<summary>Alternative syntax with leading '%' symbol</summary>**

```
pattern int-list = %[]int
```

</details>

You can also create list patterns that match a list of known length:

```
pattern pair = [int, str]

# true
([1, "a"] match pair)


pattern two_pairs = [ [int, str], [int, str] ]

# true
([ [1, "a"], [2, "b"] ] match two_pairs)
```

## String Patterns

Inox allows you to describe string patterns that are easier to read than regex
expressions.

```
# matches empty strings and strings containing only the 'a' character.
%str('a'+)

# matches any string containing only 'a's.
%str('a'+)

# matches any string that starts with a 'a' followed by zero or more 'b's.
%str('a' 'b'*)

# matches any string that starts with a 'a' followed by zero or more 'b's and 'c's.
%str('a' (|'b' | 'c')*)

# shorthand syntax for string patterns only containing a union.
%str(| 'b' | 'c')
```

String patterns can be composed thanks to named patterns:

```
pattern domain = "@mail.com"
pattern email-address = (("user1" | "user2") domain)
```

### Recursive String Patterns


Recursive string patterns are defined by putting a `@` symbol in front of the
pattern. ⚠️ This feature still needs some bug fixes. Also **recursive string patterns are pretty limited and slow: don't use
them to check/parse complex strings, use real parsers instead.**

```
pattern json-list = @ %str( 
    '[' 
        (| atomic-json-val
         | json-val 
         | ((json-val ",")* json-val) 
        )? 
    ']'
)

pattern json-val = @ %str( (| json-list | atomic-json-val ) )
pattern atomic-json-val = "1"
```

## Union Patterns

```
pattern int_or_str = | int | str

# true
(1 match int_or_str)

# true
("a" match int_or_str)

# adding parentheses allows the pattern union to span several lines.
pattern int_or_str = (
    | int 
    | str
)
```

ℹ️ A value is matched by an union pattern if it matches **at least one** of the
union's cases.

## Pattern Namespaces

Pattern namespaces are containers for storing a group of related patterns.

```
pnamespace ints. = {
    tiny_int: %int(0..10)
    small_int: %int(0..50)
}

# true
(1 match ints.tiny_int) 

# true
(20 match ints.small_int) 

# assign the namespace %ints to a variable
namespace = %ints.
```

<details>

<summary>Forbidden definition locations</summary>

Pattern namespaces can only be defined at the top level before any function declaration, and before any reference to a function declared further below.
```
# ok
pnamespace a. = {int: %int}

fn f(){}

# not allowed: the definition is after a function declaration.
pnamespace b. = {int: %int}
```

```
f()

# not allowed: the definition is after a call to a function that is declared further below.
pnamespace a. = {int: %int}

fn f(){}
```

</details>

## Path Patterns

- Alway start with either `%/`, `%./` or `%../`
- Path patterns that end with `/...` are **prefix patterns**, all the other
  patterns are **glob patterns**.
- If a pattern contains special characters such as `'['` or `'{'`, it should be
  quoted with backticks (**\`**):
  ```
  %/`[a-z].txt`   # valid
  %/[a-z].txt     # invalid
  ```

<details>

**<summary>Prefix path patterns</summary>**

A prefix path pattern match any path that contains its prefix.

| pattern     | value                  | match ? |
| ----------- | ---------------------- | ------- |
| `%/...`     | `/`                    | yes     |
| `%/...`     | `/file.txt`            | yes     |
| `%/...`     | `/dir/`                | yes     |
| `%/...`     | `/dir/file.txt`        | yes     |
| ---         | ---                    | ---     |
| `%/dir/...` | `/`                    | no      |
| `%/dir/...` | `/file.txt`            | no      |
| `%/dir/...` | `/dir`                 | no      |
| `%/dir/...` | `/dir/`                | yes     |
| `%/dir/...` | `/dir/file.txt`        | yes     |
| `%/dir/...` | `/dir/subdir/`         | yes     |
| `%/dir/...` | `/dir/subdir/file.txt` | yes     |

</details>

<details>

**<summary>Glob patterns</summary>**

https://en.wikipedia.org/wiki/Glob_(programming)

| pattern         | value           | match ? |
| --------------- | --------------- | ------- |
| `%/*`           | `/`             | yes     |
| `%/*`           | `/file.txt`     | yes     |
| `%/*`           | `/dir/`         | no      |
| `%/*`           | `/dir/file.txt` | no      |
| `%/*`           | `/`             | yes     |
| `%/*`           | `/file.txt`     | yes     |
| `%/*`           | `/dir/`         | no      |
| `%/*`           | `/dir/file.txt` | no      |
| ---             | ---             | ---     |
| `%/*.txt`       | `/`             | yes     |
| `%/*.txt`       | `/file.txt`     | yes     |
| `%/*.txt`       | `/file.json`    | no      |
| `%/*.txt`       | `/dir/file.txt` | no      |
| ---             | ---             | ---     |
| `%/**`          | `/`             | yes     |
| `%/**`          | `/file.txt`     | yes     |
| `%/**`          | `/dir/`         | yes     |
| `%/**`          | `/dir/file.txt` | yes     |
| `%/**`          | `/dir/subdir/`  | yes     |
| ---             | ---             | ---     |
| `%/*/**`        | `/`             | no      |
| `%/*/**`        | `/file.txt`     | no      |
| `%/*/**`        | `/dir`          | no      |
| `%/*/**`        | `/dir/`         | yes     |
| `%/*/**`        | `/dir/file.txt` | yes     |
| `%/*/**`        | `/dir/subdir/`  | yes     |
| ---             | ---             | ---     |
| `` %/`[a-z]` `` | `/`             | no      |
| `` %/`[a-z]` `` | `/a`            | yes     |
| `` %/`[a-z]` `` | `/aa`           | no      |
| `` %/`[a-z]` `` | `/0`            | no      |

</details>

> If you find an error in the documentation or a bug in the runtime, please
> create an issue.

## Host and URL Patterns

Supported schemes are: `http, https, ws, wss, ldb, odb, file, mem, s3`.

<details>

**<summary>URL patterns</summary>**

URL patterns always have at least a path, a query or a fragment.
`%https://example.com` is a **host pattern**, not a URL pattern.

- A URL pattern that ends with `/...` is a **prefix URL pattern**.
  - It matches any URL that contains its prefix
  - The query and fragment are ignored
- All other URL patterns are considered **regular**.
  - `/users/*` as the **path part** matches `/users/a, /users/b` but not
    `/users/`
  - `/users/*/` matches `/users/a/, /users/b/` but not `/users//`
  - `/users/a*` matches `/users/a, /users/ab`
  - `/users/%int` matches `/users/1, /users/12` but not `/users/a`
  - The tested URL's fragment is ignored if the pattern has no fragment or an
    empty one
  - The tested URL's query must match the pattern's query and additional
    parameters are not allowed

| pattern                           | value                                    | match ?                            |
| --------------------------------- | ---------------------------------------- | ---------------------------------- |
| `%https://example.com/...`        | `https://example.com/`                   | yes                                |
| `%https://example.com/...`        | `https://example.com/index.html`         | yes                                |
| `%https://example.com/...`        | `https://example.com/index.html?x=1`     | yes                                |
| `%https://example.com/...`        | `https://example.com/index.html#main`    | yes                                |
| `%https://example.com/...`        | `https://example.com/about/`             | yes                                |
| `%https://example.com/...`        | `https://example.com/about/company.html` | yes                                |
| `%https://example.com/...`        | `https://example.com:443/`               | no (that may change in the future) |
| `%https://example.com/...`        | `https://example.com` (Host)             | no                                 |
| ---                               | ---                                      | ---                                |
| `%https://example.com/about/...`  | `https://example.com/about/`             | yes                                |
| `%https://example.com/about/...`  | `https://example.com/about/company.html` | yes                                |
| `%https://example.com/about/...`  | `https://example.com/about`              | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/about`      | `https://example.com/about`              | yes                                |
| `%https://example.com/about`      | `https://example.com/about?`             | yes                                |
| `%https://example.com/about`      | `https://example.com/about#`             | yes                                |
| `%https://example.com/about`      | `https://example.com/about?x=1`          | yes                                |
| `%https://example.com/about`      | `https://example.com/about#main`         | yes                                |
| `%https://example.com/about`      | `https://example.com/about/`             | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/about?x=1`  | `https://example.com/about?x=1`          | yes                                |
| `%https://example.com/about?x=1`  | `https://example.com/about?x=1#main`     | yes                                |
| `%https://example.com/about?x=1`  | `https://example.com/about`              | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/about#main` | `https://example.com/about#main`         | yes                                |
| `%https://example.com/about#main` | `https://example.com/about`              | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/%int`       | `https://example.com/1`                  | yes                                |
| `%https://example.com/%int`       | `https://example.com/12`                 | yes                                |
| `%https://example.com/%int`       | `https://example.com/a`                  | no                                 |
| `%https://example.com/%int`       | `https://example.com/`                   | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/*`          | `https://example.com/a`                  | yes                                |
| `%https://example.com/*`          | `https://example.com/aa`                 | yes                                |
| `%https://example.com/*`          | `https://example.com/`                   | no                                 |
| ---                               |                                          |                                    |
| `%https://example.com/a*`         | `https://example.com/a`                  | yes                                |
| `%https://example.com/a*`         | `https://example.com/aa`                 | yes                                |
| ---                               |                                          |                                    |
| `%https://example.com/*/`         | `https://example.com/a/`                 | yes                                |
| `%https://example.com/*/`         | `https://example.com/aa/`                | yes                                |
| `%https://example.com/*/`         | `https://example.com/a`                  | no                                 |
| ---                               |                                          |                                    |

</details>

<details>

**<summary>Host patterns</summary>**

| pattern                  | value                           | match ? |
| ------------------------ | ------------------------------- | ------- |
| `%https://example.com`   | `https://example.com`           | yes     |
| `%https://example.com`   | `https://example.com:443`       | yes     |
| `%https://example.com`   | `https://example.com/` (URL)    | no      |
| ---                      | ---                             | ---     |
| `%https://**.com`        | `https://example.com`           | yes     |
| `%https://**.com`        | `https://subdomain.example.com` | yes     |
| ---                      | ---                             | ---     |
| `%https://**.com:443`    | `https://example.com`           | yes     |
| `%https://**.com:443`    | `https://example.com:443`       | yes     |
| ---                      | ---                             | ---     |
| `%https://*.com`         | `https://example.com`           | yes     |
| `%https://*.com`         | `https://subdomain.example.com` | no      |
| ---                      | ---                             | ---     |
| `%https://**example.com` | `https://example.com`           | yes     |
| `%https://**example.com` | `https://subdomain.example.com` | yes     |
| ---                      | ---                             | ---     |
| `%https://example.*`     | `https://example.com`           | yes     |
| `%https://example.*`     | `https://example.org`           | yes     |

**Scheme-less host patterns**:

`%://**.com` is a valid host pattern.

| pattern           | value                 | match ? |
| ----------------- | --------------------- | ------- |
| `%://example.com` | `://example.com`      | yes     |
| `%://example.com` | `https://example.com` | no      |

</details>

> If you find an error in the documentation or a bug in the runtime, please
> create an issue.

## Function Patterns

```
# pattern matching all functions with a single (integer) parameter and nil as return type
%fn(int)

# same pattern
%fn(arg int)

# same pattern but with an int return type
%fn(arg int) int

# like for most patterns the '%' prefix is not required in pattern definitions.
pattern int-fn = fn(int) int
```

[Back to top](#patterns)
