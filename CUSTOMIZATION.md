# Inox's Customization Model

> ⚠️ The ideas expressed in this document may not be implemented.
> You are welcome to create an issue to give your opinion.

**High Stability** is a main goal of this project. However this conflicts with the **constant need** to support more use cases,
have more features and be perfomant. Some issues can be addressed by using WASM modules (they will be supported soon). However there are certain cases such as using an alternative database engine that require a modification of the codebase. Therefore the Inox binary will probably have different **flavors**.

Tools (inox subcommands) will be provided to easily use existing, or create your own, versions/flavors of the `inox` binary. You will be able to use **several** binaries at once. For example you could have a specific version for a **service** that requires high database performance or a completely different DB engine.

## Flavors Provided By https://github.com/inoxlang/inox

[Golang Build Tags](https://www.digitalocean.com/community/tutorials/customizing-go-binaries-with-build-tags) will probably be used to logically
separate **flavor-specific** code in the Inox codebase. **The number of provided flavors will be limited.** Very specific requirements will not be addressed by the Inox project.

## Custom Flavors (community-provided or closed-source)

Creating your own flavor(s) of the `inox` binary - that is, add your own code to the codebase will be made as easy as possible.
Guides and tools will make the whole process straightforward.
A flavor could be just a bunch of files/patches.