# Pipeline

## Pipeline Statement

A pipeline statement has two or more **stages**. Each stage is an operation
generally involving the result of the previous stage. The first stage of a pipeline statement
is a **command-like call**.

In the following example the first stage `read ./file.txt` is evaluated first and its result is
stored in the anonymous variable `$`. Then the second stage (`print $`) is evaluated: it prints the content of the file.

```
read ./file.txt | print $
```

Each stage apart from the first one is either a function call (parenthesized or not) or a reference to a function (name).
For example the following statements are equivalent to the pipeline statement seen above.

```
read ./file.txt | print($)
```

```
read ./file.txt | print
```

## Pipeline Expression

A pipeline expresson has two or more **stages**. Each stage is an operation
generally involving the result of the previous stage. The first stage of a pipeline expression
is **any expression**.

In the following example the first stage `1` is evaluated first and its result is
stored in the anonymous variable `$`. Then the second stage (`add_one($)`) is evaluated. Finally the pipeline expression
returns the result of the last stage.

```
fn add_one(arg int){
    return arg + 1
}

two = 1 | add_one($)
```

Each stage apart from the first one is either a function call (**command-like calls are not supported**) or a reference to a function (name).
For example the following expression is equivalent to the pipeline expression seen above.

```
two = 1 | add_one
```

⚠️ **In some places pipeline expressions need to be parenthesized.**

```
three = 1 + 1 | add_one   # syntax error
three = 1 + (1 | add_one) # valid

(1 + 1 | add_one)   # syntax error
(1 + (1 | add_one)) # valid
```
