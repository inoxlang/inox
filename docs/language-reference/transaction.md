# Transaction

Transactions are a first class concept in Inox, they are either `readonly` or `read-write`.
Most effects are not allowed during `readonly` transactions.

The **sharable+serializable** value types such as objects and sets are
aware of transactions. A Set internally tracks the changes made by the current read-write transaction. It allows at most
one read-write to access or modify it: other read-write transactions are paused.

For more details about locking of collection types (e.g. `Set`, `Map`) refer to the [document of collections](../collections.md).

TODO: improve explanations

