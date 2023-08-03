# Routines

### Routines

Routines are mainly used for concurrent work and isolation. Each routine has its own Goroutine and state.

Embedded module:

````
routine = go {globals: .{read}, allow: {read: %https://example.com/...}} do {
    return read!(https://example.com/)
}
````

Call syntax (all permissions are inherited).
````
routine = go do f()
````

Routines can optionally be part of a "routine group" that allows easier control of multiple routines.

````
req_group = RoutineGroup()

for (1 .. 10) {
    go {group: req_group} read!(https://jsonplaceholder.typicode.com/posts)
}

results = req_group.wait_results!()
````
