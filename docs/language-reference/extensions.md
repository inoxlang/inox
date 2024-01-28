[Table of contents](./language.md)

---

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
    pending-todos: filter_iterable!(self.todos, @(!$.done))

    # extension method
    remove_done_todos: fn(){
        self.todos = filter_iterable!(self.todos, .done)
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

**Other example**

```
extend int {
    double: (self * 2)
}

one = 1
two = one::double
```

[Back to top](#extensions)
