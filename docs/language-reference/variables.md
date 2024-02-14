[Table of contents](./language.md)

---

# Variables

- [Locals](#locals)
- [Globals](#globals)
- [Multi Assignment](#multi-assignment)


There are two kinds of variables: globals & locals, local variables are declared
with the `var` keyword or with an assignment.

## Locals

```
var local1 = 1
local2 = 2
```

ℹ️ Assigning a local that is not defined is allowed but redeclaration is an
error.

Local variable declarations can have a type annotation:

```
var i int = 0
```

<details>

**<summary>Learn more about type annotations</summary>**

Type annotations are just [patterns](./patterns.md) with no leading `%` required.
The following declarations are valid:

```
var i int = 0
var i %int = 0

var object %{} = {}
var object {} = {}

var object %{a: int} = {}
var object {a: int} = {}
```

</details>

## Globals

Globals are variables or constants that are global to a **module**.\
In other terms the global scope of a module is not shared with other modules.

**Declaration of global variables**:

```
globalvar myglobal = 1

var local1 = 2
print (myglobal + local2)

# global variables cannot be shadowed by local variables ! the following line is an error.
var myglobal = 3
```

Go to the [Functions](./functions.md) section to learn more about variables &
scopes.


<details>

<summary>Forbidden declaration locations</summary>

Global variables can only be declared at the top level before any function declaration, and before any function call to a function declared further below.

```
# ok
globalvar a = {a: 1}

fn f(){}

# not allowed: the definition is after a function declaration.
globalvar b = {a: 1}
```

</details>



**Assignment of global variables**:

```
$$myglobal = 0
```

ℹ️ Assigning a global that is not defined is allowed (it will be considered as a declaration), but redeclaration is an
error.

**Global constants** are defined at the top of the file, before the manifest.

```
const (
    A = 1
)

manifest {}

print(A)
```

## Multi Assignment

Multiple variables can be assigned at once using the `assign` keyword:

```
assign first second = [1, 2]

assign first second = unknown_length_list
```

⚠️ If the number of elements is less than the number of variables the evaluation
will panic. You can use a nillable multi-assignment to avoid that:

```
assign? first second = unknown_length_list
```

If at runtime `unknown_length_list` has a single element `second` will receive a
value of `nil`.

[Back to top](#variables)
