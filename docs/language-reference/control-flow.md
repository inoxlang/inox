[Table of contents](./README.md)

---

# Control Flow

- [If statement](#if-statement--expression)
- [Switch statement](#switch-statement)
- [Switch expression](#switch-expression)
- [Match statement](#match-statement)
- [Match expression](#match-expression)
- [For statement](#for-statement)
- [For expression](#for-expression)
- [Walk statement](#walk-statement)
- [Pipe statement](#pipe-statement)

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

When the condition is a [boolean conversion expression](./unary-operations.md#boolean-conversion) the type of the converted
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


## Switch Expression

```
# "a"
result = switch 1 {
    1 => "a"
    2 => "b"
    defaultcase => "c"
}
```

Switch expressions without a `defaultcase` return `nil` if there is no match.

```
# 0
result = switch 1 {
    0 => 0
}
```

Switch expressions with no cases always return `nil`.

```
# nil
result = switch 1 {}
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

The value of the match statement is narrowed in each block except `defaultcase`'s block:

```
fn print_type(arg){

    match arg {
        %path {
            var path path = arg
        }
        %int {
            var integer int = arg
        }
        defaultcase { }
    }
}
```

## Match Expression

```
value = /a 

result = match value {
    %/a => "/a"
    %/... => "any absolute path"
    defaultcase => "?"
}

print(result)

output:
/a
```


Match expressions without a `defaultcase` return `nil` if there is no match.

```
# 0
result = match 1 {
    %string => "s"
}
```

Match expressions with no cases always return `nil`.

```
# nil
result = match 1 {}
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

## For Expression

For expressions allow you to create a [list](./serializable-data-structures.md#lists) by
evaluating an expression for each iteration.

```
double = (for n in [1, 2, 3]: (2 * n))
print(double)

output:
2
4
6
```


## Walk Statement

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

## Pipe Statement

Pipe statements are analogous to pipes in Unix but they act on the values
returned by functions, not file descriptors.

Here is an example:

```
map_iterable [{value: "a"}, {value: 1}] .value | filter_iterable $ %int
```

- in the first call we extract the .value property of several objects using the
  `map` function
- in the second call we filter the result of the previous call
  - `$` is an anonymous variable that contains the result of the previous call
  - `%int` is a pattern matching integers

Pipe expressions allow you to store the final result in a variable:

```
ints = | map_iterable [{value: "a"}, {value: 1}] .value | filter_iterable $ %int
```

[Back to top](#control-flow)
