[Back To README](./README.md)

_'I' refers to [GraphR00t](https://github.com/GraphR00t), the creator and maintainer of Inox._

## Questions You May Have

<details>

**<summary>Why isn't Inox using a container runtime such as Docker ?</summary>**


Because the long term goal of Inox is to be a **simple**, single-binary and **super stable** platform for applications written in Inoxlang  and using librairies compiled to WASM.\
Each application or service will ultimately run in a separate process:
- filesystem isolation is achieved by using virtual filesystems (meta filesystem)
- process-level access control will be achieved using [Landlock](https://landlock.io/)
- fine-grained module-level access control is already achieved by Inox's permission system
- process-level resource allocation and limitation will be implemented using cgroups
- module-level resource allocation and limitation is performed by Inox's limit system

</details>

_________


<details>

**<summary>Why isn't Inox using an embedded database engine such as SQLite ?</summary>**

SQLite is a fast embedded database engine with JSON support and virtually no configuration required.

However, implementing a custom database engine gives more control over caching, memory allocation and transactions.
My goal is to have a DB engine that is aware of the code accessing it (HTTP request handlers) in order to smartly pre-fetch and cache data. It could even support **partial deserialization**: for example if an object is stored as `{"name":"foo","value":1,"other-data":{...}}` in the database and a piece of code only requires the `name` property, only this property could be retrieved by iterating over the marshalled JSON.

The database currently uses a single-file key-value store ([a BuntDB fork](https://github.com/tidwall/buntdb)) and the serialization of most container types is not yet implemented. All data is loaded in memory but I will change that. BuntDB appends changes to the database file when they are commited. I plan to implement a simple continuous backup system on S3 by writing small files containing the changes and periodically concatenate them without any download.

**Related**:
- https://github.com/whitfin/s3-concat
- https://stackoverflow.com/a/64785907

**What about encryption ?**

Inox's database engine will support encryption in the future.

**What about use cases requiring high performance ? JSON is not fast, a binary format is a better fit.**

See [customization](CUSTOMIZATION.md).

</details>

_________

<details>

**<summary>Is Inoxlang sound ?</summary>**

No, Inoxlang is unsound. **BUT**:

- The **any** type does not disable checks like in Typescript. It is more similar to **unknow**.
- The type system is not overly complex and I don't plan to add classes or true generics*.
- Type assertions using the `assert` keyword are checked at runtime.

_\*Types like Set are kind of generic but it cannot be said that generics are implemented._

</details>

_________

<details>

**<summary>Is Inox a company ? What is the business model of Inox ?</summary>**

Inox is not a company. Please consider donating through [GitHub](https://github.com/sponsors/GraphR00t) (preferred) or [Patreon](https://patreon.com/GraphR00t) to support my work.

I have a ton of work to do on the platform and the ecosystem to make Inox truly usable. In the future I may develop [sponsorware](https://github.com/sponsorware/docs), and services that are **peripheral** to the project. 

**Note that Inox will ALWAYS be licensed under the MIT license (or similar).** If you have a question feel free to create an issue or contact me on the [Inox Discord Server](https://discord.gg/53YGx8GzgE).

</details>

_________

<details>

**<summary>When will Inox be production-ready ?</summary>**

If I receive enough donations to continue working full time I aim to release a production-ready version of Inox at the **end of 2024** or the beginning of 2025. A few complex features will still be experimental though.

_production-ready != battle-tested_

</details>

_________

<details>

**<summary>What is the state of the codebase (quality, documentation, tests) ?</summary>**

As of now, certain parts of the codebase are not optimally written, lack sufficient comments and documentation, and do not have robust test coverage. The first version (0.1) being now released, I will dedicate 20-30% of my working time to improving the overall quality, documentation, and test coverage of the codebase.

</details>

_________

<details>

**<summary>The language is slow, do you plan to improve the performance ?</summary>**

Yes, I plan to improve execution speed and memory usage. Note that some sharable data structures such as **objects** and **sets**
are lock-protected. [Structs](./docs/language-reference.md#structs) should be used to represent state when performing resource-intensive computations. Objects should be mostly used to persist data.

</details>

_________

Have a question ? Create an [issue](https://github.com/inoxlang/inox/issues/new?assignees=&labels=question&projects=&template=question.md&title=).

<details>

**<summary>Donations</summary>**

I am working full-time on Inox, please consider donating through [GitHub](https://github.com/sponsors/GraphR00t) (preferred) or [Patreon](https://patreon.com/GraphR00t). Thanks !

</details>


[Back To README](./README.md)