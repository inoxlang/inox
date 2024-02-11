[Table of contents](./language.md)

---

# Symbolic Evaluation

The symbolic evaluation of a module is a "virtual" evaluation, it performs
checks similar to those of a type checker. Throughout the Inox documentation you
may encounter the terms "type checker"/ "type checking", they correspond to the
symbolic evaluation phase.


<details>

**<summary>Is Inoxlang sound ?</summary>**

No, Inoxlang is unsound. **BUT**:

- The **any** type is not directly available to the developer, and it does not disable checks like in Typescript. It is more similar to **unknow**.
- The type system is not overly complex and I don't plan to add classes or advanced generics*.
- Type assertions using the `assert` keyword are checked at runtime.

_\*Types like Set are kind of generic but it cannot be said that generics are implemented._

</details>