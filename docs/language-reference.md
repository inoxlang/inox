[Install Inox](../README.md#installation) | [Built-in Functions](./builtin.md) |
[Project](./project.md) | [Web App Development](./web-app-development.md) |
[Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

---

# Inox Language Reference

- [Literals](#Literals)
- [Variables](#variables)
  - [Locals](#locals)
  - [Globals](#globals)
- [Operations](#operations)
  - [Binary operations](#binary-operations)
  - [Unary operations](#unary-operations)
  - [Concatenation](#concatenation-operation)
  - [Interpolation](#interpolation)
- [Data Structures](#data-structures)
  - [Lists](#lists)
  - [Objects](#objects)
  - [Tuples](#tuples)
  - [Records](#records)
  - [Treedata](#treedata)
  - [Mappings](#mappings)
  - [Dictionaries](#dictionaries)
- [Control flow](#Control-flow)
  - [If statement](#if-statement--expression)
  - [Switch statement](#switch-statement)
  - [Match statement](#match-statement)
  - [For statement](#for-statement)
  - [Walk statement](#walk-statement)
  - [Pipe statement](#pipe-statement)
- [Functions](#functions)
  - [Definitions](#function-definitions)
  - [Call](#calling-a-function)
  - ['Must' calls](#must-calls)
  - [Variadic Functions](#variadic-functions)
- [Patterns](#patterns)
  - [Named patterns](#named-patterns)
  - [Object patterns](#object-patterns)
  - [List patterns](#list-patterns)
  - [String patterns](#string-patterns)
  - [Union Patterns](#union-patterns)
  - [Pattern namespaces](#pattern-namespaces)
  - [Path Patterns](#path-patterns)
  - [Host and URL Patterns](#host-and-url-patterns)
- [Extensions](#extensions)
- [XML Expressions](#xml-expressions)
- [Modules](#modules)
  - [Module Parameters](#module-parameters)
  - [Permissions](#permissions)
  - [Execution Phases](#execution-phases)
  - [Inclusion Imports](#inclusion-imports)
  - [Module Imports](#module-imports)
  - [Limits](#limits)
  - [Main Module](#main-module)
- [Pre-Initialization](#pre-initialization)
- [Static check](#static-check)
- [Symbolic evaluation](#symbolic-evaluation)
- [Concurrency](#concurrency)
  - [LThreads](#lthreads)
  - [LThread Groups](#lthread-groups)
  - [Data Sharing](#data-sharing)
- [Databases](#databases)
  - [Schema](#database-schema)
  - [Serialization](#serialization)
  - [Access From Other Modules](#access-from-other-modules)
- [Testing](#testing)
  - [Basic](#basic)
  - [Custom Filesystem](#custom-filesystem)
  - [Program Testing](#program-testing)
- [Project Images](#project-images)
- [Structures (not implemented yet)](#structs)

> If you find an error in the documentation or a bug in the runtime, please
> create an issue.

# Literals

Here are the most commonly used literals in Inox:

- numbers with a point (.) are floating point numbers: `1.0, 2.0e3`
- numbers without a point are integers: `1, -200, 1_000`
- integer range literals: `1..3, 1..`
- boolean literals are `true` and `false`
- nil literal (it represents the absence of value): `nil`
- single line strings have double quotes: `"hello !"`
- multiline strings have backquotes:
  ```
  `first line
  second line`
  ```
- runes represent a single character, they have single quotes: `'a', '\n'`
- regex literals: `` %`a+` ``

<details>

**<summary>URL & Path literals</summary>**

- path literals represent a path in the filesystem: `/etc/passwd, /home/user/`
  - they always start with `./`, `../` or `/`
  - paths ending with `/` are directory paths
  - if the path contains spaces or delimiters such as `[` or `]` it should be
    quoted: `` /`[ ]` ``
- path pattern literals allow you to match paths
  - `%/tmp/...` matches any path starting with `/tmp/`, it's a prefix path
    pattern
  - `%./*.go` matches any file in the `./` directory that ends with `.go`, it's
    a globbing path pattern
  - ⚠️ They are values, they don't expand like when you do `ls ./*.go`
  - note: you cannot mix prefix & globbing path patterns
- URL literals: `https://example.com/index.html, https://google.com?q=inox`
- URL pattern literals:
  - URL prefix patterns: `%https://example.com/...`

</details>

<details>

**<summary>Other literals</summary>**

- host literals: `https://example.com, https://127.0.0.1, ://example.com`
- host pattern literals:
  - `%https://**.com` matches any domain or subdomain ending in .com
  - `%https://**.example.com` matches any subdomain of `example.com`
- port literals: `:80, :80/http`
- year literals: `2020y-UTC`
- date literals: `2020y-10mt-5d-5h-4m-CET`
- datetime literals represent a specific point in time:
  `2020y-10mt-5d-5h-4m-CET`
  - The location part at the end is mandatory (CET | UTC | Local | ...).
- quantity literals: `1B 2kB 10%`
- quantity range literals `1kB..1MB 1kB..`
- rate literals: `5B/s 10kB/s`
- byte slice literals: `0x[0a b3]  0b[1111 0000] 0d[120 250]`

</details>

# Variables

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

Type annotations are just [patterns](#patterns) with no leading `%` required.
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

Go to the [Functions](#functions) section to learn more about variables &
scopes.

**Assignment of global variables**:

```
$$myglobal = 0
```

ℹ️ Assigning a global that is not defined is allowed but redeclaration is an
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

# Operations

## Binary Operations

Binary operations are always parenthesized:

- integer addition: `(1 + 2)`
- integer comparison: `(1 < 2)`
- floating point addition: `(1.0 + 2.5)`
- floating point comparison: `(1.0 < 2.5)`
- deep equality: `({a: 1} == {a: 1})`
- logical operations: `(a or b)`, `(a and b)`
- exclusive-end range: `(0 ..< 2)`
- inclusive-end range: `(0 .. 2)`

ℹ️ Parentheses can be omitted around operands of **or**/**and** chains:

```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok
```

This [script](../examples/basic/binary-expressions.ix) contains most possible
binary operations.

### Match

The binary **match** operation returns true if the value on the left matches the
pattern on the right. The pattern does not require a `%` prefix, unless it's a
pattern literal.

```
object = {a: 1}

(object match {a: 1})  # the right operand is NOT an object here, it's a pattern.
(object match %{a: 1}) # equivalent to the previous line

(object match {a: int})
(object match %{a: int}) # equivalent
```

## Unary Operations

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

Boolean negation:

```
!true # false

myvar = true
!myvar # false
```

## Concatenation Operation

Concatenation of strings, byte slices and tuples is performed with a
concatenation expression.

```
# result: "ab"
concat "a" "b"

# result: 0x[00 11 22]
concat 0x[00] 0x[11 22]

# result: #[1, 2]
concat #[1] #[2]
```

**Parenthesized** concatenation expressions can span several lines:

```
(concat "start"
    "1" # comment
    "2"
    "end"
)
```

## Interpolation

### Regular Strings

```
`Hello ${name}`

`Hello ! 
I am ${name}`
```

### Checked Strings

In Inox checked strings are strings that are validated against a pattern. When
you dynamically create a checked string all the interpolations must be
explicitly typed:

```
pattern integer = %`(0|[1-9]+[0-9]*)`

pnamespace math. = {
    expr: %str( %integer (| "+" | "-") %integer)
    int: %integer
}

one = "1"
two = "2"

checked_string = %math.expr`${int:one}+${int:two}`
```

### URL Expressions

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

### Path Expressions

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

# Data Structures

## Lists

A list is a sequence of elements. You can add elements to it and change the
value of an element at a given position.

```
list = []
list.append(1)

first_elem = list[0] # index expression
list[0] = 2

list = [1, 2, 3]
first_two_elems = list[0:2] # creates a new list containing 1 and 2
```

## Objects

An object is a data structure containing properties, each property has a name
and a value.

```
object = {  
    a: 1
    "b": 2
    c: 0, d: 100ms
}

a = object.a
```

Implicit-key properties are properties that can be set without specifying a
name:

```
object = {
    1
    []
}

print(object)

output:
{
    "0": 1
    "1": []
}
```

Properties with an implicit key can be accessed thanks to an index expression,
the index should always be an integer:

```
object = {1}
one = object[0] # 1
1
```

### Methods

Function expression properties can access the current object using `self`.

```
object = {
    name: "foo"
    print: fn(){
        print(`hello I am ${self.name}`)
    }
}

object.print()
```

ℹ️ It is recommended to define methods in [extensions](#extensions), not in the
objects.

### Computed Member Expressions

Computed member expressions are member expressions where the property name is
computed at runtime:

```
object = { name: "foo" }
property_name = "name"
name = object.(property_name)
```

⚠️ Accessing properties dynamically may cause security issues, this feature will
be made more secure in the near future.

### Optional Member Expressions

```
# if obj has a `name` property the name variable receives the property's value, nil otherwise.
name = obj.?name
```

## Records

<details>

<summary>Click to expand</summary>

Records are the immutable equivalent of objects, their properties can only have
immutable values.

```
record = #{
    a: 1
    b: #{ 
        c: /tmp/
    }
}

record = #{
    a: {  } # error ! an object is mutable, it's not a valid property value for a record.
}
```

</details>

## Tuples

<details>

<summary>Click to expand</summary>

Tuples are the immutable equivalent of lists.

```
tuple = #[1, #[2, 3]]

tuple = #[1, [2, 3]] # error ! a list is mutable, it's not a valid element for a tuple.
```

</details>

## Treedata

<details>

<summary>Click to expand</summary>

A treedata value allows you to represent immutable data that has the shape of a
tree.

```
treedata "root" { 
    "first child" { 
        "grand child" 
    }   
    "second child"
    3
    4
}
```

<!-- In the shell execute the following command to see an example of treedata value ``fs.get_tree_data ./docs/`` -->

</details>

## Mappings

<details>

<summary>Click to expand</summary>

<!-- TODO: add explanation about static key entries, ... -->

A mapping maps keys and key patterns to values:

```
mapping = Mapping {
    0 => 1
    n %int => (2 * n)
    %/... => "path"
}

print mapping.compute(0)
print mapping.compute(1)
print mapping.compute(/e)

output:
1
2
path
```

</details>

## Dictionaries

<details>

<summary>Click to expand</summary>

Dictionaries are similar to objects in that they store key-value pairs, but
unlike objects, they allow keys of any data type as long as they are
representable (serializable).

```
dict = :{
    # path key
    ./a: 1

    # string key
    "./a": 2

    # integer key
    1: 3
}
```

</details>

# Control Flow

## If Statement & Expression

```
a = 1

if (a > 0){
    # ...
} else {
    # ...
}

string = (if (a > 0) "positive" else "negative or zero")

val = (if false 1) # val is nil because the condition is false
```

When the condition is a boolean conversion expression the type of the converted
value is narrowed:

```
intOrNil = ...

if intOrNil? {
    # intOrNil is an integer
} else {
    # intOrNil is nil
}
```

## Switch Statement

```
switch 1 {
    1 {
        print 1
    }
    2 {
        print 2
    }
    defaultcase { }
}

output:
1
```

## Match Statement

The match statement is similar to the switch statement but uses **patterns** as
case values. The match statement executes the block following the first pattern
matching the value.

```
value = /a 

match value {
    %/a {
        print "/a"
    }
    %/... {
        print "any absolute path"
    }
    defaultcase { }
}

output:
/a
```

## For Statement

```
for elem in [1, 2, 3] {
    print(elem)
}

output:
1
2
3
```

```
for index, elem in [1, 2, 3] {
    print(index, elem)
}

output:
0 1
1 2
2 3


for key, value in {a: 1, b: 2} {
    print(key, value)
}

output:
a 1
b 2
```

```
list = ["a", "b", "c"]
for i in (0 ..< len(list)) {
    print(i, list[i])
}

output:
0 "a"
1 "b"
2 "c"
```

```
for i in (0 .. 2) {
    print(i)
}

output:
0
1
2
```

```
for (0 .. 2) {
    print("x")
}

output:
x
x
x
```

<details>
<summary>Advanced use</summary>

Values & keys can be filtered by putting a pattern in front of the **value** and
**key** variables.

**Value filtering:**

```
for int(0..2) elem in ["a", 0, 1, 2, 3] {
    print(elem)
}

output:
0
1
2
```

**Key filtering:**

```
# filter out keys not matching the regex ^a+$.

for %`^a+$` key, value in {a: 1, aa: 2, b: 3} {
    print(key, value)
}

output:
a 1
aa 2
```

</details>

## Walk Statement

<details>
    <summary>Click to expand</summary>

**walk statements** iterate over a **walkable** value. Like in **for
statements** you can use the **break** & **continue** keywords.

```
fs.mkdir ./tempdir/ :{
    ./a.txt: ""
    ./b/: :{
        ./c.txt: ""
    }
}

walk ./tempdir/ entry {
    print(entry.path)
}

output:
./tempdir/
./tempdir/a.txt
./tempdir/b/
./tempdir/b/c.txt


# the prune statement prevents the iteration of the current directory's children
walk ./tempdir/ entry {
    if (entry.name == "b") {
        prune
    }
    print $entry.path
}

output:
./tempdir/
./tempdir/a.txt
./tempdir/b/
```

Walking over a [**treedata**](#treedata) value:

```
tree = treedata "root" {
    "child 1" {
        "grandchild"
    } 
    "child 2"
}

walk tree entry {
    print(entry)
}

output:
"root"
"child 1"
"grandchild"
"child 2"
```

</details>

## Pipe Statement

Pipe statements are analogous to pipes in Unix but they act on the values
returned by functions, not file descriptors.

Here is an example:

```
map [{value: "a"}, {value: 1}] .value | filter $ %int
```

- in the first call we extract the .value property of several objects using the
  `map` function
- in the second call we filter the result of the previous call
  - `$` is an anonymous variable that contains the result of the previous call
  - `%int` is a pattern matching integers

Pipe expressions allow you to store the final result in a variable:

```
ints = | map [{value: "a"}, {value: 1}] .value | filter $ %int
```

# Functions

There are 2 kinds of functions in Inox: normal Inox functions & native Golang
functions (that you cannot define).

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
[patterns](#patterns) with no leading `%` required. The following function
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

# Patterns

In Inox a pattern is a **runtime value** that matches values of a given kind and
shape.\
Besides the pattern [literals](#literals), there are other kinds of patterns in
Inox such as object patterns `%{a: int}`.\
Even though patterns are created at runtime, they can act as types:

```
pattern small_int = int(0..10)

# small_int is created at runtime but it can be used in type annotations:
var n small_int = 0
```

ℹ️ In summary you will mostly define **patterns**, not types.

## Named Patterns

Named patterns are equivalent to variables but for patterns, there are many
built-in named patterns such as: `int, str, bool`. Pattern definitions allow you
to declare a pattern.

```
pattern int_list = []int

# true
([1, 2, 3] match int_list) 

pattern user = {
    name: str
    friends: []str
}
```

⚠️ Named patterns cannot be reassigned.

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
pattern int_list = []int

([] match pattern) # true
([1] match pattern) # true
([1, "a"] match pattern) # false
```

<details>

**<summary>Alternative syntax with leading '%' symbol</summary>**

```
pattern int_list = %[]int
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
# matches any string containing only 'a's
%str('a'+)

# matches any string that starts with a 'a' followed by zero or more 'b's.
%str('a' 'b'*)

# matches any string that starts with a 'a' followed by zero or more 'b's and 'c's.
%str('a' (|'b' | 'c')*)
```

String patterns can be composed thanks to named patterns:

```
pattern domain = "@mail.com"
pattern email-address = (("user1" | "user2") %domain)
```

## Union Patterns

```
pattern int_or_str = | int | str

# true
(1 match int_or_str)

# true
("a" match int_or_str)
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

# Host and URL Patterns

Supported schemes are: `http, https, ws, wss, ldb, odb, file, mem, s3`.

<details>

**<summary>URL patterns</summary>**

URL patterns always have at least a path, a query or a fragment.
`%https://example.com` is a **host pattern**, not a URL pattern.

- A URL pattern that ends with `/...` is a **prefix URL pattern**.
  - It matches any URL that contains its prefix
  - The query and fragment are ignored
- All other URL patterns are considered **regular**.
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
| ---

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

# Extensions

An **extension** consists of a set of computed properties and methods that can
be accessed/called on values matching a given pattern.

```
pattern todo = {
    title: str
    done: bool
}

pattern user = {
    name: str
    todos: []todo
}

extend user {
    # computed properties (lazy)
    pending-todos: filter(self.todos, @(!$.done))

    # extension method
    remove_done_todos: fn(){
        self.todos = filter(self.todos, .done)
    }
}

# the following value matches the user pattern
var user = {
    name: "Tom"
    todos: [
        {title: "Todo 1", done: false}
        {title: "Todo 2", done: true}
    ]
}

# [{title: "Todo 1", done: false}]
pending-todos = user::pending-todos

user::remove_done_todos()

# [{title: "Todo 1", done: false}]
user.todos
```

# XML Expressions

An XML expression produces a value by passing a XML-like structure to a
namespace that interprets it:

```
string = "world"
element = html<div> Hello {string} ! </div>

# self closing tag
html<img src="..."/>
```

In the `<script>` and `<style>` tags, anything inside single brackets is not
treated as an interpolation:

```
html<html>
    <style>
        html, body {
            margin: 0;
        }
    </style>
    <script>
        const object = {a: 1}
    </script>
</html>
```

# Modules

An Inox module is a code file that starts with a manifest.

## Module Parameters

Module can take parameters, for the main module they correpond to the CLI
parameters.\
In the following module manifest two parameters are defined: **dir** and
**verbose**:

```
manifest {
    parameters: {
        # positional parameters are listed at the start
        {
            name: #dir
            pattern: %path
            rest: false
            description: "root directory of the project"
        }
        # non positional parameters
        verbose: %bool
    }
}

dir = mod-args.dir
clean-existing = mod-args.clean-existing
```

Arguments should be added after the path when executing the program:

```
inox run [...run options...] ./script.ix ./dir/ --verbose
```

## Permissions

The permissions section of the manifest lists the permissions required by the
module. Permissions represent a type of action a module is allowed (or
forbidden) to do. Most IO operations (filesystem access, HTTP requests) and
resource intensive operations (lthread creation) necessitate a permission.

**Examples:**

```
# reading any file in /home/user/ or below
manifest {
    permissions: {
        read: {
            %/home/user/...
        }
    }
}

# sending HTTP GET & POST requests to any HTTPS server
manifest {
    permissions: {
        read: {
            %https://**
        }
        write: {
            %https://**
        }
    }
}

# creating an HTTPS server listening on localhost:8080
manifest {
    permissions: {
        provide: https://localhost:808
    }
}

# reading from & writing to the database ldb://main
manifest {
    permissions: {
        read: {
            ldb://main
        }
        write: {
            ldb://main
        }
    }
}

# creating lightweight threads
manifest {
    permissions: {
        create: {
            threads: {}
        }
    }
}
```

## Execution Phases

The execution of a module has several phases:

**preparation phases**:

- **Parsing**
- [Pre-initialization](#pre-initialization)
- **Opening of Databases**
- [Static Check](#static-check)
- [Symbolic Evaluation/Check](#symbolic-evaluation)

**actual execution phases**:

- [Compilation](#compilation) (if using the [bytecode interpreter](#evaluation))
- [Evaluation](#evaluation)

## Result

Inox modules can return a value with a return statement:

```
# return-1.ix
manifest {}

return 1
```

This feature is generally used by imported modules to return a result or export
functions.

## Inclusion Imports

Inclusion imports include the content of a file in the current file. They are
useful to decompose a module or regroup pattern definitions/functions shared
between modules.

```
# main.ix
manifest {}

import ./patterns.ix

# patterns.ix
includable-chunk

pattern user = {
    name: str
    profile-picture: url
}
```

⚠️ This feature is currently in development ! File inclusion will follow strict
rules.

## Module Imports

As the name imply this language construct imports a **module**: an Inox file
that starts with a manifest. Here is a minimal example:

```
# main.ix
manifest {
    permissions: {
        read: %/...    # don't forget the read permission
    }
}

import result ./return_1.ix {}

print(result) 


# return-1.ix
manifest {}

return 1
```

Module imports starts by the creation of a new instance of the imported module.
Then the instance is executed and its result is returned.

⚠️ As a consequence, if you import the same module from two different files, the
instances will not be the same. Let's see an example.

We have the modules `/main.ix`, `/lib1.ix`, `/lib2.ix`.

- Both `main` and `lib1` import `lib2`
- `main` also imports `lib1`

```
# --- main.ix ---

manifest {
    permissions: {
        read: %/...
    }
}

import lib1 ./lib1.ix {}

# this instance of lib2 is not the same as the one in /lib1.ix.
import lib2 ./lib2.ix {}


# --- lib1.ix ---

manifest {
    permissions: {
        read: %/...
    }
}

# this instance of lib2 is not the same as the one in /main.ix.
import lib2 ./lib2.ix {}


# --- lib2.ix ---

return {
    state: {
        # ....
    }
}
```

### Arguments

As explained [here](#module-parameters) module can take parameters. When an
imported module does have parameters you have to pass arguments to it.

```
# main.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

import result ./add.ix {
    args: {1, 2}
} 

print(result) 

# add.ix
manifest {
    parameters: {
        {
            name: #first_operand
            pattern: %int
        }
        {
            name: #second_operand
            pattern: %int
        }
    }
}

return (mod-args.first_operand + mod-args.second_operand)
```

### Granting Permissions

In most cases the modules you import will require access to the filesystem or
the network. You can grant them the required permissions in the **allow**
section of the import.

> Note: in the following example IWD_PREFIX refers to a prefix path pattern
> matching the working directory

```
# main.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

import read-config ./read-config.ix {
    allow: {read: IWD_PREFIX}
}

config = read-config()
# ...


# read-config.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

return fn(){
    # ...
}
```

⁉️ So I need to write a manifest + specify permissions in **EACH** file ?\
-> No, you will typically use [inclusion imports](#inclusion-imports) for
trusted, local files. Modules are useful to provide a library or to decompose an
application in smaller parts.

## Limits

Limits limit intensive operations, there are three kinds of limits:
**[byte rate](#byte-rate-limits)**, **[simple rate](#simple-rate-limits)** &
**[total](#total-limits)**. Limits are defined in module manifests.

```
manifest {
    permissions: {
        ...
    }
    limits: {
        "fs/read": 10MB/s
        "http/req": 10x/s
    }
}
```

### Sharing

At runtime a counter will be created for each limit, the behaviour of the
counter is specific to the limit's kind. Limits defined by a module will be
shared with all of its child modules/threads. In other words when the module
defining the limit or one if its children performs an operation a shared counter
is decremented.

**Example 1 - CPU Time**

```
# ./lib.ix
manifest {}

do_intensive_operation1()
...
return ...


# ./main.ix
manifest {
    limits: {
        "execution/cpu-time": 1s
    }
}

# all CPU time spent by the lib is added to the counter of ./main.ix
import lib ./lib.ix {} 

# all CPU time spent by the child threads are added to the counter of ./main.ix
lthread = go do {
    do_intensive_operation2()
}

...
```

**Example 2 - Simultaneous Thread Count**

```
# ./main.ix
manifest {
    limits: {
        "threads/simul-instances": 2
    }
}

# lthread creation, the counter is decreased by one
lthread = go do {
    # lthread creation inside the child lthread, the counter is decreased by one
    go do {
        sleep 1s
    }
    sleep 1s
}

# at this point 2 lthreads are running, attempting to create a new one would cause an error.
...
```

### Byte Rate Limits

This kind of limit represents a number of bytes per second.\
Examples:

- `fs/read`
- `fs/write`

### Simple Rate Limits

This kind of limit represents a number of operations per second.\
Examples:

- `fs/new-file`
- `http/request`
- `object-storage/request`

### Total Limits

This kind of limit represents a total number of operations or resources.
Attempting to make an operation while the counter associated with the limit is
at zero will cause a panic.\
Examples:

- `fs/total-new-file` - the counter can only go down.
- `ws/simul-connection` - simultaneous number of WebSocket connections, the
  counter can go up & down since connections can be closed.
- `execution/cpu-time` - the counter decrements on its own, it pauses when an IO
  operation is being performed.
- `execution/total-time` - the counter decrements on its own.

## Main Module

In Inoxlang "a" **main module** does not always refer to the first module being
executed because in some cases modules can invoke other "main" modules. In
general the main module is the "main" module of "a" project.

# Pre-Initialization

The pre-initialization is the first of the module execution phases. During this
phase the `manifest` and the `preinit` block are evaluated.

```
const (
    HOST = https://localhost:8080
)

manifest {
    permissions: {
        read: HOST
    }
}
```

Any logic that has to be executed before the manifest should be written in the
`preinit` statement:

```
const (
    HOST = https://localhost:8080
)

# < no code allowed here >

preinit {
    @host = HOST
}

# < no code allowed here >

manifest {
    permissions: {
        read: @host/index.html
    }
}
```

The code in the preinit statement is heavily restricted, only a few constructs
are allowed:

- **host alias definitions**
- **pattern** and **pattern namespace definitions**
- **inclusion imports** of files subject to the same constraints as the preinit
  statement.

# Static Check

During the static check phase the code is analyzed in order to find the
following issues:

- misplaced statements
- undeclared variables or patterns
- duplicate declarations

_(and a few others)_

# Symbolic Evaluation

The symbolic evaluation of a module is a "virtual" evaluation, it performs
checks similar to those of a type checker. Throughout the Inox documentation you
may encounter the terms "type checker"/ "type checking", they correspond to the
symbolic evaluation phase.

# Compilation

TODO

# Evaluation

The evaluation is performed by either a **bytecode interpreter or** a **tree
walking interpreter**. You don't really need to understand how they work, just
remember that:

- the bytecode interpreter is the default when running a script with `inox run`
- the REPL always uses the tree walking interpreter
- the tree walking intepreter is much slower (filesystem & network operations
  are not affected)

# Concurrency

## LThreads

LThreads (lightweight threads) are mainly used for concurrent work and
isolation. Each lthread runs an Inox module in a dedicated Goroutine.\
The main way to create a lthread is by using a spawn expression:

```
thread = go do {
    # embedded module

    return read!(https://example.com/)
}

# shorthand syntax
thread = go do f()
```

### Passing Values

Values can be passed to the lthread by adding a meta value (an object) with a
**globals** section:

```
var mylocal = 1
globalvar myglobal = 2

thread = go {globals: {a: mylocal, b: myglobal}} do {
    # in the embedded module both a and b are globals.
    return (a + b)
}

# the wait_result method returns an Array with two elements: 
- the value returned by the thread (or nil on error)
- an error (or nil)
assign result err = thread.wait_result()


thread = go {globals: {a: mylocal, b: globalvar}} do idt((a + b))
```

Data sharing follows specific rules that are explained in details
[here](#data-sharing).

ℹ️ Predefined globals (print, read, write, http, fs, ...) are always inherited,
you don't need to add them to the **globals** section.

### Permissions

Lthreads created by spawn expressions inherit almost all of the permissions of
the parent module by default. The thread creation permission is removed.

You can specify what permissions are granted in the **allow** section of the
meta value:

```
# create a lthread with no permissions.
thread = go {
    allow: {}
} do {

}

# create a lthread allowed to read any file.
thread = go {
    allow: { read: %/... }
} do {
    return fs.read!(/file.txt)
}
```

## Lthread Groups

LThreads can optionally be part of a "thread group" that allows easier control
of multiple lthreads.

```
req_group = LThreadGroup()

for (1 .. 10) {
    go {group: req_group} do read!(https://jsonplaceholder.typicode.com/posts)
}

results = req_group.wait_results!()
```

## Data Sharing

Execution contexts can share & pass values with/to other execution contexts.
Most **sharable** values are either **immutable** or **lock-protected**:

```
immutable = #[1, 2, 3]
lock_protected = {a: 1}

go {globals: {immutable: immutable, lock_protected: lock_protected}} do {
    # assigning a property causes the underlying lock of the object to be acquired
    # before the mutation and to be released afterwards.
    lock_protected.a = 2
}
```

The most common immutables values are floats, integral values (ints, bytes,
...), string-like values and records & tuples. The most common lock-protected
values are objects.

**Non-sharable** values that are **clonable** are cloned when passed to another
execution context:

```
clonable = [1, 2, 3]

go {globals: {clone: clonable}} do {
    # modifying the clone does no change the original value
    clone.append(4)
}
```

### Functions

Inox functions are generally sharable unless they assign a global variable.

Go **functions** are sharable but Go **methods** are not:

```
# not sharable
method = LThreadGroup().wait_results
```

### Objects

Properties of **shared objects** are shared/cloned when accessed:

```
user = {friends: ["foo"]}

friends_copy = user.friends

# user.friends is not updated
friends_copy.append("bar")
```

The mutation of a property is achieved by using a **double-colon** syntax:

```
user::friends.append("bar")
```

In Inox Web applications it is frequent for request execution contexts to access
properties of the same object concurrently. When an object is accessed by an
execution context having an **associated transaction** all other access attempts
from other execution contexts are paused until the transaction finishes.

# Databases

Inox comes with an embedded database engine, you can define databases in the
manifest:

```
manifest {
    # permissions required by the database
    permissions: {
        read: %/databases/...
        write: %/databases/...
    }
    databases: {
        main: {
            #ldb stands for Local Database
            resource: ldb://main 
            resolution-data: /databases/main/
        }
    }
}
```

## Database Schema

The schema of an Inox Database is an [object pattern](#object-patterns), it can
be set by calling the **update_schema** method on the database:

```
pattern user = {
  name: str
}

dbs.main.update_schema(%{
    users: Set(user, #url)
})
```

⚠️ Calling **.update_schema** requires the following property in the database
description: `expected-schema-update: true`.

The current schema of the database determinates what values are accessible from
`dbs.main`.\
If the current schema has a `users` property the users will be accessible by
typing `dbs.main.users`.

You can make the typesystem pretend the schema was updated by adding the
**assert-schema** property to the database description.\
For example if the database has an empty schema, you can use the following
manifest to add the `users` property to the database value:

```
preinit {
    pattern schema = %{
        users: Set(user, #url)
    }
}

manifest {
    permissions: {
        read: %/databases/...
        write: %/databases/...
    }
    databases: {
        main: {
            resource: ldb://main 
            resolution-data: /databases/main/
            assert-schema: %schema
        }
    }
}

users = dbs.main.users
```

⚠️ At runtime the current schema of the database is matched against
`assert-schema`.\
So before executing the program you will need to add a call to
`dbs.main.update_schema` with the new schema.

## Migrations

Updating the schema often requires data updates. When this is the case
`.updated_schema` needs a second argument that describes the updates.

```
pattern new_user = {
    ...user
    new-property: int
}

dbs.main.update_schema(%{
    users: Set(new_user, #url)
}, {
   inclusions: :{
        %/users/*/new-property: 0

        # a handler can be passed instead of an initial value
        # %/users/*/new-property: fn(prev_value) => 0
    }
})
```

| Type                | Reason                                           | Value, Handler Signature                     |
| ------------------- | ------------------------------------------------ | -------------------------------------------- |
| **deletions**       | deletion of a property or element                | **nil** OR **fn(deleted_value)**             |
| **replacements**    | complete replacement of a property or element    | **value** OR **fn(prev_user user) new_user** |
| **inclusions**      | new property or element                          | **value** OR **fn(prev_value) new_user**     |
| **initializations** | initialization of a previously optional property | **value** OR **fn(prev_value) new_user**     |

ℹ️ In application logic, properties can be added to database objects even if they
are not defined in the schema.\
During a migration previous values are passed to the handlers.

## Serialization

Most Inox types (objects, lists, Sets) are serializable so no translation layer
is needed to add/retrieve objects to/from the database.

```
new_user = {name: "John"}
dbs.main.users.add(new_user)

# true
dbs.main.users.has(new_user)
```

Since most Inox types are serializable they cannot contain transient values.

```
object = {
  # error: non-serializable values are not allowed as initial values for properties of serializables
  lthread: go do {
    return 1
  }
}

# same error
list = [  
  go do { return 1 }
]
```

## Access From Other Modules

If the `/main.ix` module defines a `ldb://main` database, imported modules can
access the database with the following manifest:

```
manifest { 
    databases: /main.ix
    permissions: {
        read: {
            ldb://main
        }
        # you can also add the write permission if necessary
    }
}

for user in dbs.main.users {
    print(user)
}
```

ℹ️ The module defining the databases is automatically granted access to the
database.

⚠️ Permissions still need to be granted in the import statement.

# Testing

Inox comes with a powerful testing engine that is deeply integrated with the
Inox runtime.

## Basic

A single test is defined by a **testcase** statement. Test suites are defined
using **testsuite** statements and can be nested.

```
manifest {}

testsuite "my test suite" {
    testcase "1 < 2" {
        assert (1 < 2)
    }
}

testsuite "my test suite" {
    testsuite "my sub test suite" {
        testcase "1 < 2" {
            assert (1 < 2)
        }
    }
}
```

Tests are allowed in any Inox file but it is recommended to write them in
`*.spec.ix` files. The modules whose filename matches `*.spec.ix` are granted
the following permissions by default in test mode:

- **read, write, delete all files**
- **read, write, delete all values in the ldb://main database**
- **read any http(s) resource**
- **create lightweight threads (always required for testing)**

The metadata and parameters of the test suites and test cases are specified in
an object:

```
manifest {}


testsuite ({
    name: "my test suite"
}) {

    testcase({ name: "1 < 2"}) {

    }

}
```

## Custom Filesystem

Test suites and test cases can be configured to use a short-lived filesystem:

```
manifest {}

snapshot = fs.new_snapshot{
    files: :{
        ./file1.txt: "content 1"
        ./dir/: :{
            ./file2.txt: "content 2"
        }
    }
}

testsuite ({
    # a filesystem will be created from the snapshot for each test suite and test case.
    fs: snapshot
}) {

    assert fs.exists(/file1.txt)
    fs.rm(/file1.txt)

    testcase {
        # no error
        assert fs.exists(/file1.txt)
        fs.rm(/file1.txt)
    }

    testcase {
        # no error
        assert fs.exists(/file1.txt)
    }
}
```

Test suites can pass a copy of their filesystem to subtests:

```
testsuite ({
    fs: snapshot
    pass-live-fs-copy-to-subtests: true
}) {
    fs.rm(/file1.txt)

    testcase {
        # error
        assert fs.exists(/file1.txt)
    }

    testcase {
        # modifications done by test cases have no effect for subsequent tests
        # because they are given a copy of the test suite's filesystem.
        fs.rm(/file2.txt)
    }

    testcase {
        # no error
        assert fs.exists(/file2.txt)

        # error
        assert fs.exists(/file1.txt)
    }

}
```

## Program Testing

Inox's testing engine is able to launch an Inox program/application. Test suites
& test cases accept a **program** parameter that is inherited by subtests. The
program is launched for each test case in a short-lived filesystem.

```
manifest {
    permissions: {
        provide: https://localhost:8080
    }
}

testsuite({
    program: /web-app.ix
}) {
    testcase {
        assert http.exists(https://localhost:8080/)
    }

    testcase {
        assert http.exists(https://localhost:8080/about)
    }
}
```

The short-lived filesystem is created from the current project's
[base image](#project-images).

**Database initialization**:

The main database of the program can be initialized by specifying a schema and
some initial data:

```
manifest {
    permissions: {
        provide: https://localhost:8080
    }
}

pattern user = {
    name: str
}

testsuite({
    program: /web-app.ix
    main-db-schema: %{
        users: Set(user, #url)
    }
    # initial data
    main-db-migrations: {
        inclusions: :{
            %/users: []
        }
    }
}) {
    testcase "user creation" {
        http.post!(https://localhost:8080/users, {
            name: "new user"
        })

        db = __test.program.dbs.main
        users = get_at_most(10, db.users)

        assert (len(users) == 1)
        assert (users[0].name == "new user")
    }
}
```

# Project Images

A **project image** is a filesystem snapshot + some metadata about the project.
Projects have several types of **images**, the most important one is the **base
image**.

The base image contains:

- all `.ix` files
- all files in `/static/`

# Structs

⚠️ This feature is not implemented yet and is subject to change.

Structs are transient values that only exist in the stack on in a temporary
storage managed by a module.\
Unlike most core Inox types such as objects & lists, structs are not necessarily
serializable but will be more memory efficient.\
Accessing the field of a struct will be faster than accessing a property/element
of other Inox types.

The main usage of structs will be storing and representing temporary state.

```
struct Position2D {
    x int
    y int
}

struct LexerState {
    index int
    tokens Array(Token)
}

struct Token {
    type TokenType
    value string
}
```
