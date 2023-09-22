[Install Inox](../README.md#installation) | [Built-in Functions](./builtin.md) | [Project](./project.md) | [Web App Development](./web-app-development.md) | [Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

-----

# Inox Language Reference

- [Literals](#Literals)
- [Variables](#variables)
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
    - [Udata](#udata)
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
- [Patterns](#patterns)
    - [Named patterns](#named-patterns)
    - [Object patterns](#object-patterns)
    - [List patterns](#list-patterns)
    - [String patterns](#string-patterns)
    - [Union Patterns](#union-patterns)
    - [Pattern namespaces](#pattern-namespaces)
- [XML Expressions](#xml-expressions)
- [Modules](#modules)
    - [Module Parameters](#module-parameters)
    - [Permissions](#permissions)
    - [Execution Phases](#execution-phases)
    - [Inclusion Imports](#inclusion-imports)
    - [Module Imports](#module-imports)
- [Static check](#static-check)
- [Symbolic evaluation](#symbolic-evaluation)
- [Concurrency](#concurrency)
    - [LThreads](#lthreads)
    - [Data Sharing](#data-sharing)
- [Databases](#databases)
    - [Schema](#database-schema)
    - [Serialization](#serialization)
    - [Access From Other Modules](#access-from-other-modules)

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
- regex literals: ``%`a+` ``

<details>

**<summary>URL & Path literals</summary>**

- path literals represent a path in the filesystem: `/etc/passwd, /home/user/`
    - they always start with `./`, `../` or `/`
    - paths ending with `/` are directory paths
    - if the path contains spaces or delimiters such as `[` or `]` you can use a quoted path: `` /`[ ]` ``
- path pattern literals allow you match a path
    - `%/tmp/...` matches any path starting with `/tmp/`, it's a prefix path pattern
    - `%./*.go` matches any file in the `./` directory that ends with `.go`, it's a globbing path pattern
    - ⚠️ They are values, they don't expand like when you do `ls ./*.go`
    - note: you cannot mix prefix & globbing path patterns for now
- URL literals: `https://example.com/index.html, https://google.com?q=inox`
- URL pattern literals:
    - URL prefix patterns: `%https://example.com/...`

</details>

<details>

**<summary>Other literals</summary>**

- host literals: `https://example.com, https://127.0.0.1`
- host pattern literals:
    - `%https://**.com` matches any domain or subdomain ending in .com
    - `%https://**.example.com` matches any subdomain of `example.com`
- port literals: `:80, :80/http`
- date literals represent a specific point in time: `2020y-10mt-5d-CET`, `2020y-10mt-5d-5h-4m-CET`
    - The location part (CET | UTC | Local | ...) at the end is mandatory.
- quantity literals: `1B 2kB 10%`
- quantity range literals `1kB..1MB 1kB..`
- rate literals: `5B/s 10kB/s`
- byte slice literals: `0x[0a b3]  0b[1111 0000] 0d[120 250]`

</details>



# Variables

There are two kinds of variables: globals & locals, local variables are declared with the `var` keyword or with an assignment.
## Locals

```
var local1 = 1
local2 = 2
```

ℹ️ Assigning a local that is not defined is allowed but redeclaration is an error.

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

Declaration of global variables:
```
$$myglobal = 1 # note: the syntax might change in the near future.

var local1 = 2
print (myglobal + local2)

# global variables cannot be shadowed by local variables ! the following line is an error.
var myglobal = 3
```

Go to the [Functions](#functions) section to learn more about variables & scopes.



Global constants are defined at the top of the file, before the manifest.
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

⚠️ If the number of elements is less than the number of variables the evaluation will panic.
You can use a nillable multi-assignment to avoid that:

```
assign? first second = unknown_length_list
```

If at runtime `unknown_length_list` has a single element `second` will receive a value of `nil`.

# Operations


## Binary Operations

Binary operations are always parenthesized:

- integer addition: `(1 + 2)`
- integer comparison: `(1 < 2)`
- floating point addition: `(1.0 + 2.5)`
- floating point comparison: `(1.0 < 2.5)`
- deep equality: `({a: 1} == {a: 1})`
- logical operations: `(a or b)`, `(a and b)`

ℹ️ Parentheses can be omitted around operands of **or**/**and** chains:
```
(a or b or c)       # ok
(a < b or c < d)    # ok

(a or b and c)      # error: 'or' and 'and' cannot be mixed in the same chain
(a or (b and c))    # ok
((a or b) and c)    # ok 
```

This [script](../examples/basic/binary-expressions.ix) contains most possible binary operations.

## Unary Operations 

A number negation is always parenthesized
```
(- 1.0)
(- 1)
```

Boolean negation:
```
!true # false

myvar = true
!myvar # false
```

## Concatenation Operation

Concatenation of strings, byte slices and tuples is performed with a concatenation expression.
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
`Hello {{name}}`

`Hello ! 
I am {{name}}`
```

### Checked Strings

In Inox checked strings are strings that are validated against a pattern. When you dynamically
create a checked string all the interpolations must be explicitly typed:
```
%integer = %`(0|[1-9]+[0-9]*)`

%math. = {
    expr: %str( %integer (| "+" | "-") %integer)
    int: %integer
}

one = "1"
two = "2"

checked_string = %math.expr`{{int:one}}+{{int:two}}`
```


### URL Expressions

When you dynamically create URLs the interpolations are restricted based on their location (path, query).

```
https://example.com/api/{path}/?x={x}
```

- interpolations before the **'?'** are **path** interpolations
    - the strings/characters **..** | **\*** | **\\** | **?** | **#** are forbidden
    - **':'** is forbidden at the start of the finalized path (after all interpolations have been evaluated)
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

⚠️ Some sequences such as '..' are allowed in the path but not in the interpolation !
```
# ok
/home/user/dir/..

path = /../../etc/passwd
/home/user/{path}
# error: result of a path interpolation should not contain any of the following substrings: '..', '\', '*', '?'
```


# Data Structures

## Lists

A list is a sequence of elements, you can add elements to it and change the value of an element at a given position.

```
list = []
append(list, 1)

first_elem = list[0] # index expression
list[0] = 2

list = [1, 2, 3]
first_two_elems = list[0:2] # creates a new list containing 1 and 2
```

## Objects

An object is a data structure containing properties, each property has a name and a value.

```
object = {  
    a: 1
    "b": 2
    c: 0, d: 100ms
}

a = object.a
```

Implicit-key properties are properties that can be set without specifying a name:
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

Properties with an implicit key can be accessed thanks to an index expression, the index should always be an integer:
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
        print(`hello I am {{self.name}}`)
    }
}

object.print()
```

### Computed Member Expressions

Computed member expressions are member expressions where the property name is computed at runtime:

```
object = { name: "foo" }
property_name = "name"
name = object.(property_name)
```

⚠️ Accessing properties dynamically may cause security issues, this feature will be made more secure in the near future.

## Records

<details>

<summary>Click to expand</summary>

Records are the immutable equivalent of objects, their properties can only have immutable values.
```
record = #{
    a: 1
    b: #{ 
        c: /tmp/
    }
}

record = #{
    a: {  } # error ! an object is mutable, it's not a valid property value for a record
}
```

</details>


## Tuples

<details>

<summary>Click to expand</summary>

Tuples are the immutable equivalent of lists.
```
tuple = #[1, #[2, 3]]

tuple = #[1, [2, 3]] # error ! a list is mutable, it's not a valid element for a tuple
```

</details>


## Udata

<details>

<summary>Click to expand</summary>

A udata value allows you to represent immutable data that has the shape of a tree.

```
udata "root" { 
    "first child" { 
        "grand child" 
    }   
    "second child"
    3
    4
}
```

<!-- In the shell execute the following command to see an example of udata value ``fs.get_tree_data ./docs/`` -->

</details>


## Mappings

<details>

<summary>Click to expand</summary>

<!-- TODO: add explanation about static key entries, ... -->

A mapping maps keys and pattern of keys to values

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

Dictionaries are similar to objects in that they store key-value pairs, but unlike objects, 
they allow keys of any data type as long as they are representable (serializable).

```
dict = :{
    ./a: 1
    "./a": 2
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

When the condition is a boolean conversion expression the type of the converted value is narrowed:
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

The match statement is similar to the switch statement but uses **patterns** as case values.
The match statement executes the block following the first pattern matching the value.

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

<details>
<summary>Advanced use</summary>

Values & keys can be filtered by putting a pattern in front of the **value** and **key** variables.

**Value filtering:**

```
for %int(0..2) elem in ["a", 0, 1, 2, 3] {
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

**walk statements** iterate over a **walkable** value. Like in **for statements** 
you can use the **break** & **continue** keywords.

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

Walking over a [**udata**](#udata) value:

```
tree = udata "root" {
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

Pipe statements are analogous to pipes in Unix but they act on the values returned by functions, not 
file descriptors.

Here is an example:

```
map [{value: "a"}, {value: 1}] .value | filter $ %int
```

- in the first call we extract the .value property of several objects using the `map` function
- in the second call we filter the result of the previous call
  - `$` is an anonymous variable that contains the result of the previous call
  - `%int` is a pattern matching integers


Pipe expressions allows you to store the final result in a variable:
```
ints = | map [{value: "a"}, {value: 1}] .value | filter $ %int
```

# Functions

There are 2 kinds of functions in Inox: normal Inox functions & native Golang functions (that you cannot define).

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

<details>

**<summary>Learn more about type annotations</summary>**

As for local variable declarations, type annotations are just [patterns](#patterns) with no leading `%` required.
The following function declarations are valid:
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


Local variables are local to a function's scope or to the module's top local scope.
Blocks might be introduced in the future.

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

Since the `g` function has a single parameter you can call it with a special syntax in addition to the previous ones.
```
g{a: 1}   # equivalent to g({a: 1})

g"string" # equivalent to g("a")
```

# Patterns

Besides the pattern [literals](#literals) there are other kinds of patterns in Inox.

## Named Patterns

Named patterns are equivalent to variables but for patterns, there are many built-in named patterns such as: `int, str, bool`.
Pattern definitions allow you to declare a pattern.

```
%int_list = []int

# true
([1, 2, 3] match int_list) 

%user = {
    name: str
    friends: []str
}
```

⚠️ Named patterns cannot be reassigned.

Some named patterns are callable, for example if you want a pattern that matches all integers in the range 0..10 you can do the following:
```
pattern = %int(0..10)
```

Creating a named pattern `%user` does not prevent you to name a variable `user`:
```
%user = {
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
%object_pattern = {
    name: str
}

# true
({name: "John"} match object_pattern) 

%other_pattern = {
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

%user = {
    name: str
}

# true
({name: "John", additional_prop: 0} match user) 
```

## List Patterns

The syntax for patterns that match a list with **elements of the same type** (only integers, only strings, etc.) is as follows:
```
%int_list = []int

([] match pattern) # true
([1] match pattern) # true
([1, "a"] match pattern) # false
```

<details>

**<summary>Alternative syntax with leading '%' symbol</summary>**
```
%int_list = %[]int
```
</details>

You can also create list patterns that match a list of known length:

```
%pair = [int, str]

# true
([1, "a"] match pair)


%two_pairs = [ [int, str], [int, str] ]

# true
([ [1, "a"], [2, "b"] ] match two_pairs)
```

## String Patterns

Inox allows you to describe string patterns that are easier to read than regex expressions.

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
%domain = "@mail.com"
%email-address = (("user1" | "user2") %domain)
```

## Union Patterns

```
%int_or_str = | int | str

# true
(1 match %int_or_str)

# true
("a" match %int_or_str)
```

ℹ️ A value is matched by an union pattern if it matches **at least one** of the union's cases.

## Pattern Namespaces

Pattern namespaces are containers for storing a group of related patterns.

```
%ints. = {
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


# XML Expressions

An XML expression is the interpretation of a XML-like value by a namespace:
```
string = "world"
element = html<div> Hello {string} ! </div>

# self closing tag
html<img src="..."/>
```

# Modules

An Inox module is a code file that starts with a manifest.

## Module Parameters

Module can take parameters, for the main module they correpond to the CLI parameters.\
In the following module manifest two parameters are defined: **dir** and **verbose**:

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

The permissions section of the manifest lists the permissions required by the module. 
Permissions represent a type of action a module is allowed (or forbidden) to do. 
Most IO operations (filesystem access, HTTP requests) and resource intensive operations (lthread creation) necessitate a permission.

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
- **Parsing**
- [Static Check](#static-check)
- [Symbolic Evaluation/Check](#symbolic-evaluation)
- [Compilation](#compilation) (if using the [bytecode interpreter](#evaluation))
- [Evaluation](#evaluation)

## Result

Inox modules can return a value with a return statement:
```
# return-1.ix
manifest {}

return 1
```

This feature is generally used by imported modules to return a result or export functions.

## Inclusion Imports

Inclusion imports include the content of a file in the current file.
They are useful to decompose a module or regroup pattern definitions/functions shared between modules.

```
# main.ix
manifest {}

import ./patterns.ix

# patterns.ix
includable-chunk

%user = {
    name: str
    profile-picture: url
}
```

⚠️ This feature is currently in development ! File inclusion will follow strict rules.

## Module Imports

As the name imply this language construct imports a **module**: an Inox file that starts with a manifest.
Here is a minimal example:
```
# main.ix
manifest {
    permissions: {
        read: IWD_PREFIX    # don't forget the read permission
    }
}

import result ./return_1.ix {}

print(result) 


# return-1.ix
manifest {}

return 1
```

### Arguments

As explained [here](#module-parameters) module can take parameters. 
When an imported module does have parameters you have to pass arguments to it.

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

In most cases the modules you import will require access to the filesystem or the network.
You can grant them the required permissions in the **allow** section of the import.

> Note: in the following example IWD_PREFIX refers to a prefix path pattern matching the working directory

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
-> No, you will typically use [inclusion imports](#inclusion-imports) for trusted, local files. Modules are useful to 
provide a library or to decompose an application in smaller parts.

# Static Check

During the static check phase the code is analyzed in order to find the following issues:
- misplaced statements
- undeclared variables or patterns
- duplicate declarations

*(and a few others)*

# Symbolic Evaluation

The symbolic evaluation of a module is a "virtual" evaluation, it performs checks similar to those of a type checker.
Throughout the Inox documentation you may encounter the terms "type checker"/ "type checking", they correspond to the symbolic evaluation phase.

# Compilation

TODO

# Evaluation

The evaluation is performed by either a **bytecode interpreter or** a **tree walking interpreter**. You don't really need to understand
how they work, just remember that:
- the bytecode interpreter is the default when running a script with `inox run`
- the REPL always uses the tree walking interpreter
- the tree walking intepreter is much slower (filesystem & network operations are not affected)


# Concurrency

## LThreads

LThreads (lightweight threads) are mainly used for concurrent work and isolation. Each lthread runs an Inox module in a dedicated Goroutine.

**Embedded module:**

````
thread = go {allow: {read: %https://example.com/...}} do {
    # embedded module

    return read!(https://example.com/)
}
````

Call syntax (all permissions are inherited):
````
thread = go do f()
````

LThreads can optionally be part of a "thread group" that allows easier control of multiple lthreads.

````
req_group = LThreadGroup()

for (1 .. 10) {
    go {group: req_group} read!(https://jsonplaceholder.typicode.com/posts)
}

results = req_group.wait_results!()
````

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

The most common immutables values are floats, integral values (ints, bytes, ...), string-like values and records & tuples.
The most common lock-protected values are objects.

**Non-sharable** values that are **clonable** are cloned when passed to another execution context:

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

In Inox Web applications it is frequent for request execution contexts to access properties of the same object concurrently. 
When an object is accessed by an execution context having an **associated transaction** all other access attempts
from other execution contexts are paused until the transaction finishes.


# Databases

Inox comes with an embedded database engine, you can define databases in the manifest:
```
manifest {
    # permissions required by the database
    permissions: {
        read: %/databases/...
        write: %/databases/...
    }
    databases: {
        main: {
            resource: ldb://main  #ldb stands for Local Database
            resolution-data: /databases/main/
        }
    }
}
```

## Database Schema

The schema of an Inox Database is an [object pattern](#object-patterns), it can be
set by calling the **update_schema** method on the database:
```
%user = {
  name: str
}

dbs.main.update_schema(%{
    users: Set(user, #url)
})
```

⚠️ calling **.update_schema** requires the following property in the db description: **expected-schema-update: true**

## Migrations

Updating the schema often requires data updates, when this is the case .updated_schema needs a 
second argument that describes the updates.

```
%new_user = {
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

| Type |  Reason  | Value, Handler Signature |
| ----------- | -----------  | ----------- |
| **deletions** | deletion of a property or element | **nil** OR **fn(deleted_value)** |
| **replacements** | complete replacement of a property or element |**value** OR **fn(prev_user user) new_user** |
| **inclusions** | new property or element |**value** OR **fn(prev_value) new_user** |
| **initializations** | initialization of a previously optional property |**value** OR **fn(prev_value) new_user** |

ℹ️ In application logic properties can be added to database objects even if they are not defined in the schema.\
During a migration previous values are passed to the handlers.

## Serialization

Most Inox types (objects, lists, Sets) are serializable so no translation layer is
needed to add/retrieve objects to/from the database. 

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

If the `/main.ix` module defines a `ldb://main` database, imported modules can access the database with
the following manifest:

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


ℹ️ The module defining the databases is automatically granted access to the database.

⚠️ Permissions still need to be granted in the import statement.