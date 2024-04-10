# Internal

This folder contains most of the code for the `inox` binary.

**Folders and Packages**

_From more relevant to less relevant._

[core](./core/README.md)
- Core Inox types
- Bytecode + tree walking interpreters
- Code analysis
- Runtime components (context, global state, module)
- Default core patterns and pattern namespaces (`%int`, `%object`, `%inox.`, ...)

[parse](./parse/README.md)
- Inoxlang AST Types
- Inoxlang parser
- AST utils

[globals](./globals/README.md)
- Globals (print, sleep, ...)
- Namespaces (http, fs, ...)
- Default limits

[inoxprocess](./inoxprocess/README.md)
- Control server
- Control client
- Inox binary upgrade logic
- Process-level access control using Landlock
- ExternalFS (WIP)

[inoxd](./inoxd/README.md)
- Service installation (Systemd)
- Daemon

[config](./config/README.md)
- Process wide configuration

[project](./project/README.md)
- Project registry
- Project type and logic
- Scaffolding (e.g. templates)

[projectserver](./projectserver/README.md)
- Standard LSP method handlers
- Custom LSP method handlers
- Language-agnostic LSP logic & types

[localdb](./localdb/README.md)
- Local database.

[obsdb](./obsdb/database.go)
- Database based on object storage with on-disk cache.

[filekv](./filekv/kv.go)
- Single file Key-Value store, it's a thin wrapper around https://github.com/etcd-io/bbolt

[buntdb](./buntdb/buntdb.go)
- BuntDB fork (https://github.com/tidwall/buntdb)

[mod](./mod/execution.go)
- Module execution
- Risk score computation

[third_party_stable](./third_party_stable/README.md)

- This folder contains several third party packages of small size that are
  stable or don't need updates.

[compressarch](./compressarch/README.md)
- Wrapper functions for untarring tarballs.
- Wrapper functions for unzipping gzip archives.

[jonsiter](./jonsiter/README.md)
- JSON stream type
- JSON iterator type 

[metricsperf](./metricsperf/README.md)
- Profiling of the CPU, memory, mutexes and goroutines

[reqratelimit](./reqratelimit/README.md)
- Rate limiting of network requests

[memds (in-memory data structures)](./memds/README.md)
- Small zero-allocation bitset type (BitSet32) and generic graph type (Graph32)
- Generic directed graph types
- Generic queue types (array queue)

[css](./css/README.md)
- Parsing
- Minification
- Import graph computation
- Database of standard properties
- Tailwind subset
- Basic bundler
- Var-based utility classes

[deno](./deno/README.md)
- JS Services (startup + RPC)
- Deno binary download and installation

[sourcecontrol](./sourcecontrol/README.md)
- Git API subset with virtual filesystem support

[htmx](./htmx/README.md)
- htmx.js
- HTMX extensions
- Data on headers (e.g. `HX-Target`)

[hyperscript](./hyperscript/README.md)
- hyperscript.js subset generation
- AST Node types and parsers
- Help data

[codecompletion](./codecompletion/README.md)
- Code completion

[hack](./hack/zerolog.go)
- Small reflection hacks to inspect and modify some zerolog types.

[help](./help/README.md)
- Help on builtins
- Help on language
- Functions to retrieve help by topic name or value (Go function)
- Help message formatting

[learn](./learn/tutorials.go)
- Tutorial data

[prettyprint](./prettyprint/pretty_print.go)
- Pretty-printing configuration type
- Pretty-printing helper type

[commonfmt](./commonfmt/README.md)
- Functions to format some general messages and values.

[afs (Abstract Filesystem)](./afs/abstract_fs.go)
- Filesystem interface
- File interfaces

[inoxconsts](./inoxconsts/consts.go)
- File related constants
- Database scheme names
- Default port constants

[netaddr](./netaddr/README.md)
- Types representing remote IP addresses
- Helper functions to filter interface IPs

[testconfig](./testconfig/config.go)
- Configuration of the codebase tests

[utils](./utils/README.md)
- Path utils
- Regex transformation
- Unix process control
- Various small utility functions