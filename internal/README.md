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

**filekv**            
- single file Key-Value store (BuntDB fork).

**local_db**
- Local database.

**obs_db (wip)**
- Database based on object storage with on-disk cache.

**config**

**project**

**project_server**