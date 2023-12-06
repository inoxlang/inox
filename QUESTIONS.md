[Back To README](./README.md)

_'I' refers to [GraphR00t](https://github.com/GraphR00t), the creator and maintainer of Inox._

## Questions You May Have

<details>

**<summary>Why isn't Inox using a container runtime such as Docker ?</summary>**

</details>

Because the long term goal of Inox is to be a **simple**, single-binary and **super stable** platform for applications written in Inoxlang + WASM.\
Each application or service will ultimately run in a separate process:
- filesystem isolation is achieved by using virtual filesystems (meta filesystem)
- process-level access control will be achieved using [Landlock](https://landlock.io/)
- fine-grained module-level access control is already achieved by Inox's permission system
- process-level resource allocation and limitation will be implemented using cgroups
- module-level resource allocation and limitation is peformed by Inox's limit system

<details>

**<summary>What is the business model of Inox ?</summary>**

</details>


<details>

**<summary>When will Inox be production-ready ?</summary>**

If I receive enough donations to continue working full time I aim to release a production-ready version of Inox at the **end of 2024** or the beginning of 2025. A few complex features will still be experimental though.

_production-ready != battle-tested_

</details>


Have a question ? Create an [issue](https://github.com/inoxlang/inox/issues/new?assignees=&labels=question&projects=&template=question.md&title=).

I am working full-time on Inox, please consider donating through [GitHub](https://github.com/sponsors/GraphR00t) (preferred) or [Patreon](https://patreon.com/GraphR00t). Thanks !

[Back To README](./README.md)