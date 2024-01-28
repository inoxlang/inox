[Table of contents](./language.md)

---

# Concurrency

- [LThreads](#lthreads)
- [LThread Groups](#lthread-groups)
- [Data Sharing](#data-sharing)

## LThreads

LThreads (lightweight threads) are mainly used for concurrent work and
isolation. Each lthread runs an Inox module in a dedicated goroutine.\
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
[here](./concurrency.md#data-sharing).

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

The most common immutable values are floats, integral values (ints, bytes, ...),
string-like values and records & tuples. The most common lock-protected values
are objects.

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

[Back to top](#concurrency)

