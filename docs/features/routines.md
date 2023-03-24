# Routines

### Routines

Routines are mainly used for concurrent work and isolation. Each routine has its own goroutine and state.

Embedded module:

````
routine = go {http: http} {
    return http.get!(https://example.com/)
} allow { 
    use: {globals: ["http"]} 
}
````

Call syntax (all permissions are inherited).
````
routine = go nil f()
````

Routines can optionally be part of a "routine group" that allows easier control of multiple routines.

````
req_group = RoutineGroup()

for (1 .. 10) {
    go req_group nil read!(https://jsonplaceholder.typicode.com/posts)
}

results = req_group.wait_results!()
````
