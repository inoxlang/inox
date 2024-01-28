# Inox Coding Style

## Naming

### Constants

```
const (
     API_ENDPOINT = https://api.example.com/users
)
```

### Variable Names

```
variable_name = 1
```

### Function Names

Function names should be in the **snake_case** style.

```
fn compute_sum(){
  ...
}

sum = compute_sum()


fn new_user(name str) user {
    return {
        name: name
    }
}

user = new_user("Foo")
```

### Object Properties

- **kebab-case** for property names (values).
- **snake_case** for methods.

```
config = { 
    save-cookies: true

    save_cokies: fn(){
        ...
    }
}
```

### Pattern Names

Pattern names should be in the **kebab-case**.

```
pattern log-level = | "warn" | "info" | "debug"

pattern user = {
    name: str
    friends: Set(%ldb://main/users/%ulid)
    unread-notifications: []ulid
}
```

### Struct Names

[Structs](./language-reference/language.md#structs) are transient values. They are not intended to be persisted, contrary
to serializable types such as **Objects**. Struct names should follow the **PascalCase** style in order to distinguish
them from serializable types.

```
struct Player {
    data user
}

struct ParsingState {
    index int
}
```

---