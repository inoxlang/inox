# Host Aliases

Host aliases are primarily used to create URL interpolations.

```
# host alias definition
@host = https://localhost

url = @host/index.html
```

Hosts aliases are global to a **module**, they are not shared with other modules.

<details>

<summary>Forbidden definition locations</summary>

Host aliases can only be defined at the top level before any function declaration,and before any reference to a function declared further below.

```
# ok
@hostA = https://localhost

fn f(){}

# not allowed: the definition is after a function declaration.
@hostB = https://localhost
```

</details>

