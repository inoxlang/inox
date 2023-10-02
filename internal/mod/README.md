# Mod Package

The tests for this package are located in [../globals/module_execution_test.go](../globals/module_execution_test.go).

## Module Preparation

Module preparation is implemented in preparation.go, it consists of several steps:
- Parsing
- Pre-initialization
- Context Creation
- Global State Creation
- Static Checks
- Symbolic Evaluation (typechecking)

### Parsing

Recursive parsing of the main module and its imports.

### Pre-initialization

The pre-initialization is the checking and creation of the main module's manifest.

1.  the pre-init block is statically checked (if present).
2.  the manifest's object literal is statically checked.
3.  pre-evaluate the env section of the manifest.
4.  pre-evaluate the preinit-files section of the manifest.
5.  read & parse the preinit-files using the provided .PreinitFilesystem.
6.  evaluate & define the global constants (const ....).
7.  evaluate the preinit block.
8.  evaluate the manifest's object literal.
9.  create the manifest.

### Context Creation

A context containing all the core pattern types (int, str, ...) is created.
The most relevant inputs are:
- the permissions listed in the manifest
- the limits listed in the manifest
- the host resolution data specified in the manifest
- the **parent context** (host resolution data and limits are inherited)

### Global State Creation

[implementation](../globals/default_state.go)

The global state of the main module is created it is initialized
with the default globals (variables, functions & namespaces).


### Static Checks

During this phase the code is analyzed in order to find the following issues:
- misplaced statements
- undeclared variables or patterns
- duplicate declarations

*(and a few others)*

# Symbolic Evaluation

The symbolic evaluation of a module is a "virtual" evaluation, it performs checks similar to those of a type checker.
Throughout the Inox documentation you may encounter the terms "type checker"/ "type checking", they correspond to the 
symbolic evaluation phase.
