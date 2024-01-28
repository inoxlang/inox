[Table of contents](./language.md)

---

# Functions

- [Definitions](#function-definitions)
- [Call](#calling-a-function)
- ['Must' calls](#must-calls)
- [Variadic Functions](#variadic-functions)


There are 2 kinds of functions in Inox: normal Inox functions that you can
define, and native Golang functions.

## Function Definitions

Functions in Inox can be declared in the global scope with the following syntax:

```
fn hello(a, b){
    print("hello", a, b)
    return 0
}
```

Parameters and return value of a function can have a type annotation:

```
fn add(a int, b int) int {
    return (a + b)
}
```

ℹ️ Parameters that don't have a type annotation defaults to the **any** type.
This type is similar to **unknown** in Typescript.

<details>

**<summary>Learn more about type annotations</summary>**

As for local variable declarations, type annotations are just
[patterns](./patterns.md) with no leading `%` required. The following function
declarations are valid:

```
fn add(a int, b int) int {
    return (a + b)
}
# same as
fn add(a %int, b %int) %int {
    return (a + b)
}

fn add(a {a: int}, b {a: int}) ({a: int}) {
    return (a + b)
}
# same as
fn add(a %{a: int}, b %{a: int}) %{a: int} {
    return (a + b)
}
```

</details>

Local variables are local to a function's scope or to the module's top local
scope. Blocks might be introduced in the future.

```
fn f(){
    var a = 1
    if true {
        var a = 2 # error ! 'a' is already declared
    }
}
```

## Calling a Function

Let's define some functions.

```
fn f(a, b){
    # ...
}

fn g(arg){
    # ...
}
```

You can call `f` with parentheses or with a command-like syntax:

```
result = f(1, 2)

f 1 2 # this syntax is mostly used in the REPL
```

Since the `g` function has a single parameter it can also be called with the
following shorthand syntaxs:

```
g{a: 1}   # equivalent to g({a: 1})

g"string" # equivalent to g("string")
```

### 'Must' Calls

**'must' calls** are special calls that cause a panic if there is an error. If
there is no error the returned value is transformed:

- (error|nil) -> nil
- Array(1, (error|nil)) -> 1
- Array(1, 2, (error|nil)) -> Array(1, 2)

**Native (Golang) functions**:

A Go function is considered to have failed if the last return value is a non-nil
error.\
Let's see an example: `unhex` is a Go function decoding a hexadecimal string
that has the following return type (byte-slice, error). The result is returned
as an `Array(byte-slice, error | nil)`.

```
# normal call: a value of type Array(byte-slice, (error | nil)) is returned.
assign bytes error = unhex("...")

# must call: a value of type byte-slice is returned.
bytes = unhex!("...")
```

**Inox functions**:

An Inox function is considered to have failed if it returns an error or an Array
whose last element is an error.

```
fn f() (| error | nil) {
    ... some logic ...
    if issue {
        return Error("error")
    }
    return nil
}

# normal call: a value of type (error | nil) is returned.
err = f()

# must call: on error the runtime panics, otherwise nil is returned.
nil_value = f!()
```

```
fn g() {
    ... some logic ...
    if issue {
        return Array(-1, Error("error"))
    }

    return Array(1, nil)
}

# normal call: a value of type Array(int, (error | nil)) is returned.
assign int err = g()

# must call: a value of type int is returned.
int = g!()
```

> If you find an error in the documentation or a bug in the runtime, please
> create an issue.

## Variadic Functions

```
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

## Readonly Parameters (WIP)

Putting `readonly` in front of a pattern prevents the mutation of values
matching it. For now this is only supported for parameter patterns (parameter
types).

```
fn f(integers readonly []int){
    # error
    integers.append(1)

    # error
    integers[0] = 1
}
```

⚠️ This feature is still in development.

[Back to top](#)
