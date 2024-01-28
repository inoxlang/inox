[Table of contents](./language.md)

---

# Literals

Here are the most commonly used literals in Inox:

- integers: `1, -200, 1_000`
- floating point numbers: `1.0, 2.0e3`
- integer range literals: `1..3, 1..`
- float range literals: `1.0..3.0, 1.0..`
- boolean literals are `true` and `false`.
- the `nil` literal represents the absence of value.
- single line strings: `"hello !"`
- multiline strings have backquotes:
  ```
  `first line
  second line`
  ```
- runes represent a single character: `'a', '\n'`
- regex literals: `` %`a+` ``


## URL & Path literals

- path literals represent a path in the filesystem: `/etc/passwd, /home/user/`
  - they always start with `./`, `../` or `/`
  - paths ending with `/` are directory paths
  - if the path contains spaces or delimiters such as `[` or `]` it should be
    quoted: `` /`[ ]` ``.
- path pattern literals allow you to match paths
  - `%/tmp/...` matches any path starting with `/tmp/`, it's a prefix path
    pattern
  - `%./*.go` matches any file in the `./` directory that ends with `.go`, it's
    a globbing path pattern.
  - ⚠️ They are values, they don't expand like when you do `ls ./*.go`
  - learn more [here](./patterns.md#path-patterns)
- URL literals: `https://example.com/index.html, https://google.com?q=inox`
- URL pattern literals:
  - URL prefix patterns: `%https://example.com/...`
  - learn more [here](./patterns.md#host-and-url-patterns)

## Other Literals

- host literals: `https://example.com, https://127.0.0.1, ://example.com`
- host pattern literals:
  - `%https://**.com` matches any domain or subdomain ending in `.com`.
  - `%https://**.example.com` matches any subdomain of `example.com`
- port literals: `:80, :80/http`
- year literals: `2020y-UTC`
- date literals: `2020y-10mt-5d-5h-4m-CET`
- datetime literals represent a specific point in time:
  `2020y-10mt-5d-5h-4m-CET`
  - The location part at the end is mandatory (CET | UTC | Local | ...).
- quantity literals: `1B 2kB 10%`
- quantity range literals `1kB..1MB 1kB..`
- rate literals: `5B/s 10kB/s`
- byte slice literals: `0x[0a b3]  0b[1111 0000] 0d[120 250]`
- property name literals: `.name .age` 
- long value-path literals: `.data.name` 


</details>

[Back to top](#literals)

