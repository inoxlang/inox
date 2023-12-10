# Internal

This folder contains most of the code for the `inox` binary.

**Relevant Folders and Packages:**

**core**

- core Inox types
- bytecode + tree walking interpreters
- code analysis
- runtime components (context, global state, module)

**parse**

- Inoxlang AST Types
- Inoxlang parser
- AST utils

**globals**

- globals (print, sleep, ...)
- namespaces (http, fs, ...)
- default limits

**mod**

- module execution.

**inoxprocess**

- control server
- control client
- ExternalFS

**inoxd**

**filekv**

- single file Key-Value store (BuntDB fork).

**localdb**

- Local database.

**obsdb (wip)**

- Database based on object storage with on-disk cache.

**config**

**project**

**projectserver**

**third_party_stable**

- This folder contains several third party packages of small size that are
  stable or don't need updates.
