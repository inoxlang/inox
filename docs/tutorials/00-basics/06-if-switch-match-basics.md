# If, Switch, Match 

```
manifest {}

a = 0

# if statement
if (a == 0) {
    print("a == 0")
} else {
    print("a != 0")
}

# if expression
zero = (if (a == 0) 0 else 1)

b = 1

# switch statement
switch b {
    0 {
        print("b == 0")
    }
    1 {
        print("b == 1")
    }
    defaultcase {
        print("b != 0 and b != 1")
    }
}

# The match statement is similar to the switch statement but uses patterns as case values. 
# It executes the block following the first pattern matching the value.

c = 2

match c {
    %int(0..2) {
        print "c is in the range 0..2"
    }
    %int {
        print "c is an integer"
    }
    defaultcase { 
        print "c is not an integer"
    }
}
```