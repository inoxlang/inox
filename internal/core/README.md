# Core

This package contains most the code for the Inox Runtime, the type checking logic is in the **symbolic/** package.

- Tree Walk Interpreter
    - **tree_walk.go**
- Bytecode Interpreter (inspired from https://github.com/d5/tengo.)
    - **compiler.go**
    - **vm.go**
- Static Check
    - **static_check.go**
- Core Value Types
    - **value.go**
    - **number.go**
    - **quantity.go**
    - **data_structure.go**
- Core Pattern Types
    - **pattern.go**
    - **string_pattern.go**
- Module
    - **module.go**
    - **module_import.go**
    - **manifest.go**
- Context & Security
    - **context.go**
    - **permissions.go**
    - **limit.go**
    - **token_bucket.go**
- Permissions
- Secrets
    - **secrets.go**
- Database
    - **database.go**
- Debugger
    - **debug.go**
    - **debug_types.go**
- Serialization / Deserialization
    - **write_representation.go**
    - **write_json_representation.go**
    - **parse_representation.go**
    - **parse_json_representation.go**
    - **json_schema.go**

## Inox Runtime Architecture

### High Level View

Each Inox module is executed by a dedicated [interpreter](./docs/language-reference.md#evaluation).

```mermaid
graph TD
    Interpreter0[[Interpreter N]]
    Interpreter0 --> |runs| Mod

    Interpreter1[[Interpreter N+1]] --> |runs| ChildMod

ChildMod(Child Module)
DBs[(Databases)]
VFs[(Filesystem)]

subgraph Mod[Module]
  Context(Context)
  IncludesFiles(Included Chunks)
end

subgraph ChildMod[Child Module]
  ChildContext(Child Context)
end


Mod(Module)
Mod --- ChildMod; 
Mod --- DBs
ChildMod --- DBs
Mod --- VFs
ChildMod --- VFs
Context -.->|Controls| ChildContext
```

### Global State

Each module instance has itw own **global state**.
The global state holds the state of the module instance:
- global variables.
- the module instance's manifest (immutable).
- the module instances's [context](./#context).
- databases accessible by the module instance.
- a reference to the project the module is part of.
- a reference to the module definition (immutable).

### Context

Each module instance has its own context.\
A context is analogous to a `context.Context` in Golang's stdlib: 
when the context is cancelled all ancestor contexts are cancelled as well.

#### Creation

Most relevant inputs come from the module's manifest:
- list of permissions required by the module.
- list of limits specified by the module.
- list of database configurations specified by the module (owned databases).
- host definitions (resolution data) specified by the module.

Another relevant input is the parent context. In most cases a context have a parent context; 
when a context has a parent additional checks are performed:
- all permissions required by the module should be also granted to the parent.
- limits specified by the module must be as or more restrictive than the parent context's limits.
- no host definition should override a host defined by the parent's context.

Hosts defined by the parent context and limits are inherited.

#### Sequence Diagram for Permission Checks

```mermaid
sequenceDiagram
    Module->>Context: Do I have the [ read /file.txt ] permission ?
    Context-->>Module: ✅ Yes, you can continue execution

    Module->>Context: Do I have the [ write /file.txt ] permission ?
    Context-->>Module: ❌ No, raise an error ! (stop execution)  
```

#### Sequence Diagram for Rate Limiting

```mermaid
sequenceDiagram
    Module->>Context: I am about to do an HTTP Request (IO)
    Context->>CPU Time Limiter: Pause the auto decrementation
    Context->>HTTP Req. Limiter: Remove 1 token
    Note right of HTTP Req. Limiter: ✅ There is one token left.<br/>I take it and I return immediately.
    Context->>CPU Time Limiter: Resume the decrementation

    Module->>Context: I am starting an IO operation
    Context->>CPU Time Limiter: Pause the decrementation

    Module->>Context: The IO operation is finished
    Context->>CPU Time Limiter: Resume the decrementation

    Module->>Context: I am about to do an HTTP Request
    Context->>CPU Time Limiter: Pause the decrementation
    Context->>HTTP Req. Limiter: Remove 1 token
    Note right of HTTP Req. Limiter: ⏲️ There are no tokens left.<br/>I wait for the bucket to refill a bit<br/>and I take 1 token.
    Context->>CPU Time Limiter: Resume the decrementation

    Module->>Context: I am starting an IO operation
    Context->>CPU Time Limiter: Pause the decrementation

    Module->>Context: The IO operation is finished
    Context->>CPU Time Limiter: Resume the decrementation
```


#### Sequence Diagram for Total Limiting

```mermaid
sequenceDiagram
    Module->>Context: I am about to establish a Websocket Connection
    Context->>CPU Time Limiter: Pause the decrementation
    Context->>Websocket Conn. Limiter: Remove 1 token
    Note right of Websocket Conn. Limiter: ✅ There is one token left.<br/>I take it and I return immediately.
    Context->>CPU Time Limiter: Resume the decrementation

    Module->>Context: (After a few minutes) The connection is closed.
    Context->>Websocket Conn. Limiter: Give back 1 token
  
    Module->>Context: I am about to establish a Websocket Connection [Same as previously]
    Note right of Context: Same as previously
    Module->>Context: I am about to establish another Websocket Connection

    Context->>CPU Time Limiter: Pause the decrementation
    Context->>Websocket Conn. Limiter: Remove 1 token
    Note right of Websocket Conn. Limiter: ❌ There are no tokens left ! Panic !
    Websocket Conn. Limiter-->>Context: ❌ raising panic
    Context-->>Module: ❌ raising panic
```

<details>
<summary>Note</summary>
Obviously the context knowns nothing about HTTP requests, Websocket Connections and all other IO operations.

The module informs the context with a simple call:
```
context.Take("<simultaneous websocket connection limit>", 1)
```
</details>

**Limiters**

```mermaid
graph TD
    Limiters("Limiters (one per limit)") --> OwnTokenBuckets(Own Token Buckets) & SharedTokenBuckets(Shared Token Buckets)
    Ctx -.->|Stops when Done| ChildCtx
    Ctx(Context)
  
    ChildCtx --> ChildLimiters(Child's Limiters) --> SharedTokenBuckets
    ChildLimiters --> ChildOwnTokenBuckets(Child's Token Buckets)

    SharedTokenBuckets & OwnTokenBuckets -.->|Can Stop| Ctx

subgraph ChildCtx["Child Context(s)"]
  ChildLimiters
  ChildOwnTokenBuckets
end
```
