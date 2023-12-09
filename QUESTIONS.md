[Back To README](./README.md)

_'I' refers to [GraphR00t](https://github.com/GraphR00t), the creator and maintainer of Inox._

## Questions You May Have

<details>

**<summary>Why isn't Inox using a container runtime such as Docker ?</summary>**


Because the long term goal of Inox is to be a **simple**, single-binary and **super stable** platform for applications written in Inoxlang + WASM.\
Each application or service will ultimately run in a separate process:
- filesystem isolation is achieved by using virtual filesystems (meta filesystem)
- process-level access control will be achieved using [Landlock](https://landlock.io/)
- fine-grained module-level access control is already achieved by Inox's permission system
- process-level resource allocation and limitation will be implemented using cgroups
- module-level resource allocation and limitation is performed by Inox's limit system

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

Inox is not a company. I am working full-time on Inox and releasing the source code under the MIT license.\
Please consider donating through [GitHub](https://github.com/sponsors/GraphR00t) (preferred) or [Patreon](https://patreon.com/GraphR00t).

I may develop closed-source services AROUND the project if I earn almost nothing in sponsorship.
**Inox will always be licensed under the MIT license.**
</details>

_________

<details>

**<summary>Why are contributors required to sign a Contributor Licensing Agreement ?</summary>**

**Definition of CLA**: https://yahoo.github.io/oss-guide/docs/resources/what-is-cla.html \
**Additional context**: https://news.ycombinator.com/item?id=28923633 (comments)

The [CLA](./.legal/CLA/CLA.md) is present to protect me and the project from legal issues. I know 
some people are against CLAs or consider having a CLA useless but I prefer to have one because:
- It requires contributors to know what they are allowed to do, even if it is obvious (e.g. don't include proprietary code).
- Even if having the CLA only reduces the risks by 50% it's worth it.
- I may develop closed-source services AROUND the project if I earn almost nothing in sponsorship. **Inox will always be licensed under the MIT license.**

**By signing the CLA you do NOT GRANT me the right to include your contribution if I change the type of license.**\
**If you want to propose a change to the CLA feel free to create an issue, or contact me on Inox's Discord Server.**

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

Have a question ? Create an [issue](https://github.com/inoxlang/inox/issues/new?assignees=&labels=question&projects=&template=question.md&title=).

<details>

**<summary>Donations</summary>**

I am working full-time on Inox, please consider donating through [GitHub](https://github.com/sponsors/GraphR00t) (preferred) or [Patreon](https://patreon.com/GraphR00t). Thanks !

</details>


[Back To README](./README.md)