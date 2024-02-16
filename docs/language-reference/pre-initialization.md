[Table of contents](./README.md)

---

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

- **pattern** and **pattern namespace definitions**
- **inclusion imports** of files subject to the same constraints as the preinit
  statement.

[Back to top](#pre-initialization)
