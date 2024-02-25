# Project Server Package

This package contains the code for the Inox Project Server.
The Project Server is basically an LSP server with extra features:
- Custom methods for [debugging](./debug_methods.go) (Debug Adapter Protocol)
- Custom methods for [accessing filesystems](./filesystem_methods.go)
- Custom methods for [accessing project secrets](./secrets_methods.go)
- Custom methods for [creating and opening projects](./project_methods.go)
- Custom methods for [production management](./prod_methods.go)
- Custom methods for [retrieving learning data](./learning_methods.go) (e.g. tutorials)

Subpackages:

- **jsonrpc**
- **logs**
- **lsp**: language-agnostic logic & types

## Architecture

### Current (temporary)

```mermaid
graph TB

Spawner(inoxd or user) --> |$ inox project-server -config='...'| InoxBinary


subgraph InoxBinary[Inox Binary]
    direction TB

    ProjectServer
    NodeAgent


    ProjectServer --> |asks to deploy/stop apps| NodeAgent

    NodeAgent --> InoxRuntime1
    NodeAgent --> InoxRuntime2

end

ProjectServer[Project Server] --> |stores data in| ProjectsDir
InoxRuntime1[Inox Runtime - App 1] --> |stores data in| ProdDir
InoxRuntime2[Inox Runtime - App 2] --> |stores data in| ProdDir


ProjectsDir(/var/lib/inoxd/projects)
ProdDir(/var/lib/inoxd/prod)

```

**The next version is way more secure and resilient.**

### Next

In this version every important component runs in a separate `inox` process.

```mermaid
graph TB

Inoxd(inoxd) --> |$ inox project-server -config='...'| ProjectServer(Project Server)
Inoxd --> |spawns| NodeAgent
NodeAgent("Node Agent \n [uses cgroups]") --> |creates process| DeployedApp1(Deployed Application 1)
NodeAgent --> |creates process| DeployedApp2(Deployed Application 2)
NodeAgent --> |creates process| ServiceModule(Separate Service of App 1)
ServiceModule -..- DeployedApp1
DeployedApp1 --> |stores data in| ProdDir
DeployedApp2 --> |stores data in| ProdDir

ProjectServer[Project Server] --> |stores data in| ProjectsDir
ProjectServer --> |asks to deploy/stop apps| NodeAgent

ProjectsDir(/var/lib/inoxd/projects)
ProdDir(/var/lib/inoxd/prod)
```

_The next version may be slightly different from what is planned here._
