[Table of contents](./language.md)

---

# Evaluation

The evaluation is performed by either a **bytecode interpreter or** a **tree
walking interpreter**. You don't really need to understand how they work, just
remember that:

- the bytecode interpreter is the default when running a script with `inox run`
- the REPL always uses the tree walking interpreter
- the tree walking intepreter is much slower (filesystem & network operations
  are not affected)

[Back to top](#evaluation)
