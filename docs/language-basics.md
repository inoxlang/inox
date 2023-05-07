# Inox Basics

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
    - [Udata](#udata)
    - [Mappings](#mappings)
    - [Dictionaries](#dictionaries)
- [Control flow](#Control-flow)
    - [If statement](#if-statement--expression)
    - [Switch statement](#switch-statement)
    - [Match statement](#match-statement)
    - [For statement](#for-statement)
- [Functions](#functions)
    - [Definitions](#function-definitions)
    - [Call](#calling-a-function)
- [Patterns](#patterns)
    - [Object patterns](#object-patterns)
    - [List patterns](#list-patterns)
    - [Named patterns](#named-patterns)
    - [Pattern namespaces](#pattern-namespaces)
    - [String patterns](#string-patterns)
- [Modules](#modules)
- [Static check](#static-check)
- [Symbolic evaluation](#symbolic-evaluation)

# Literals

- numbers with a point (.) are floating point numbers: `1.0, 2.0e3`
- numbers without a point are integers: `1, -200`
- boolean literals are `true` and `false`
- nil literal (it represents the absence of value): `nil`
- single line strings have double quotes: `"hello !"`
- multiline strings have backquotes:
    ``` 
    `first line
    second line`
    ```
- runes represent a single character, they have single quotes: `'a', '\n'`
- path literals represent a path in the filesystem: `/etc/passwd, /home/user/`
    - they always start with `./`, `../` or `/`
    - paths ending with `/` are directory paths
    - if the path contains spaces or delimiters such as `[` or `]` you can use a quoted path: `` /`[ ]` ``
- path pattern literals allow you match a path
    - `%/tmp/...` matches any path starting with `/tmp/`, it's a prefix path pattern
    - `%./*.go` matches any file in the `./` directory that ends with `.go`, it's a globbing path pattern
    - ⚠️ They are values, they don't expand like when you do `ls ./*.go`
    - note: you cannot mix prefix & globbing path patterns for now
- host literals: `https://example.com, https://127.0.0.1`
- host pattern literals:
    - `%https://**.com` matches any domain or subdomain ending in .com
    - `%https://**.example.com` matches any subdomain of `example.com`
- URL literals: `https://example.com/index.html, https://google.com?q=inox`
- URL pattern literals:
    - URL prefix patterns: `%https://example.com/...`
- port literals: `:80, :80/http`
- date literals represent a specific point in time: `2020y-10mt-5d-CET`, `2020y-10mt-5d-5h-4m-CET`
    - The location part (CET | UTC | Local | ...) at the end is mandatory.
- quantity literals: `1B`, `2kB`, `10%`
- rate literals: `5B/s`, `10kB/s`
- byte slice literals: `0x[0a b3]`, `0b[1111 0000]`, `0d[120 250]`
- regex literals: ``%`a+` ``

# Variables

There are two kinds of variables: globals & locals, local variables can be declared with the `var` keyword.
Assigning a local that is not defined is allowed but redeclaration is an error.

## Locals

```
var local1 = 1
local2 = 2
```

Local variable declarations can have a type annotation:
```
var a %int = 0
```

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



Global constants are defined at the top of file, before the manifest.
```
const (
    A = 1
)

manifest {}

print(A)
```


# Operations


## Binary Operations

Binary operations are always parenthesized:

- integer addition: `(1 + 2)`
- integer comparison: `(1 < 2)`
- floating point addition: `(1.0 + 2.5)`
- floating point comparison: `(1.0 < 2.5)`
- deep equality: `({a: 1} == {a: 1})`
- logical operations: `(a or b)`, `(a and b)`


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

For now "normal" strings cannot be interpolated but the feature is coming soon, the interpolation
of checked strings, URLs & paths is already implemented.

### Checked Strings

In Inox checked strings are strings that are validated against a pattern. When you dynamically
create a checked string all the interpolations must be explicitly typed.

<img src="./img/query-injection.png"></img>

### URL Expressions

URLs are part of the language, when you dynamically create URLs the interpolations are restricted.

<img src="./img/url-injection.png"></img>

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

### Path Expressions

```
path = /.bashrc     # you can also use the path ./.bashrc or a string
/home/user/{path}
# result: /home/user/.bashrc
```

⚠️ Some sequence characters such as '..' are allowed in the path but not in the interpolation !
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

Properties can be set without specifying a name, they are called implicit key properties.
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

## Records

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


## Tuples

Tuples are the immutable equivalent of lists.
```
tuple = #[1, #[2, 3]]

tuple = #[1, [2, 3]] # error ! a list is mutable, it's not a valid element for a tuple
```

## Udata

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

## Mappings

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

## Dictionaries

Dictionaries are similar to objects in that they store key-value pairs, but unlike objects, 
they allow keys of any data type as long as they are representable (serializable).

```
dict = :{
    ./a: 1
    "./a": 2
    1: 3
}
```

# Control Flow

## If Statement & Expression

```
if (a < 0){

} else {

}

a = (if (a < 0) "a" else "b")
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
}

output:
1
```

## Match Statement

The match statement is similar to the switch statement but uses patterns as case values.
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
```

# Functions

There are 2 kinds of functions in Inox: normal Inox functions & native Golang functions (that you cannot define).

## Functions Definitions

Functions in Inox can be declared in the global scope with the following syntax:

```
fn hello(a, b){
    print("hello", a, b)
    return 0
}
```

Parameters and return value of a function can have a type annotation:

```
fn add(a %int, b %int) %int {
    return (a + b)
}
```

Local variables are "local" to a function's scope or to the module's top local scope.
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

You can call `f` with parenthesis or with a command-like syntax: 
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

## Object Patterns

```
object_pattern = %{
    name: %str
}

# true
({name: "John"} match object_pattern) 

# false
({name: "John", additional_prop: 0} match object_pattern) 
```

By default object patterns are "exact": they don't accept additional properties.
You can create an "inexact" object pattern by adding '...' in an object pattern

```
object_pattern = %{
    name: %str
    ...
}

# true
({name: "John", additional_prop: 0} match object_pattern) 
```

## List Patterns

List patterns matching a list with elements of the same shape have the following syntax:
```
pattern = %[]%int
([] match pattern) # true
([1] match pattern) # true
([1, "a"] match pattern) # false
```

You can also create list patterns that match a list of known length:

```
pattern = %[%int, %str]

# true
([1, "a"] match pattern) 
```

## Named Patterns

Named patterns are equivalent to variables but for patterns, there are many builtin named patterns such as: `%int, %str, %bool`.\
Pattern definitions allow you to "declare" a pattern like you declare a variable but with a pattern identifier.

```
%int_list = %[]%int

# true
([1, 2, 3] match %int_list) 
```

⚠️ Named patterns cannot be reassigned.

Some named patterns are 'callable', for example if you want a pattern that matches all integers in the range 0..10 you can do the following:
```
pattern = %int(0..10)
```

## Pattern Namespaces

Pattern namespaces are containers for storing a group of related patterns.

```
%ints. = #{
    tiny_int: %int(0..10)
    small_int: %int(0..50)
}

# true
(1 match %ints.tiny_int) 

# true
(20 match %ints.small_int) 

namespace = %ints.
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

# Modules

An Inox module is a code file that starts with a manifest.

## Execution Phases

The execution of a module has several phases:
- parsing
- [static check](#static-check)
- [symbolic evaluation/check](#symbolic-evaluation)
- [compilation](#compilation) (if using [bytecode interpreter](#evaluation))
- [evaluation](#evaluation-phase)

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
- the tree walking interpreter is always used when using the REPL
- the tree walking intepreter is much slower (filesystem & network operations are not affected)
