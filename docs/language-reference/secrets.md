# Secrets

Secrets are special Inox values, they can only be created by defining an
**environment variable** with a pattern like %secret-string or by storing a
[project secret](../docs/project.md#project-secrets).

- The content of the secret is **hidden** when printed or logged.
- Secrets are not serializable so they cannot be included in HTTP responses.
- A comparison involving a secret always returns **false**.

```
manifest {
    ...
    env: %{
        API_KEY: %secret-string
    }
    ...
}
API_KEY = env.initial.API_KEY
```

<details>

**<summary>Demo of project secrets</summary>**

![project secrets demo](https://github.com/inoxlang/inox/assets/113632189/55f23134-e289-4f78-bd26-693bbc75c8ea)

</details>