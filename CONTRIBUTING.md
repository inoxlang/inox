# Contribution

👉 **if you want to fix a typo or improve an error message, you can write a
comment in this [issue](https://github.com/inoxlang/inox/issues/4)**.

## Guidelines

Please before working on the codebase make sure you read [FUTURE.md](./FUTURE.md).

Contributing to Inox requires that you agree to the [Developer Certificate of Origin](https://developercertificate.org/) by 
adding `Signed-off-by: username <email address>` in the last line of your commit messages.
This can be easily automated: https://www.secondstate.io/articles/dco/.

It is recommended to avoid the following changes:
- adding a feature that will be used by only a few Inox projects
- adding a dependency that is large or has a copyleft license
- adding a dependency whose features can be easily reimplemented in the Inox repository
- adding code without at least a few tests
- modifying the **core** package

## Tests

**The code you add should be tested.** Try to test all packages that depend on your changes.

### Test a Single Package

```
go test -race -count=1 ./internal/<pkg> -timeout=3m
```

Run the tests again with the race detector disabled:

```
go test -count=1 ./internal/<pkg> -timeout=2m
```

### Test All Packages

All tests can be run with the following command with:

```
go test -race -count=1 -p=1 ./... -timeout=3m
```

Run all tests again with the race detector disabled:

```
go test -count=1 -p=1 ./... -timeout=2m
```

If you have Chrome installed you can set the env var
RUN_BROWSER_AUTOMATION_EXAMPLES to "true".

If you have a S3 bucket with read & write access you can the set the env
variables read in the following [file](internal/globals/s3_ns/fs_test.go).

If you have a Cloudflare Account you can the set the env variables read in the
following [file](internal/project/secrets_test.go).

## Save Memory Profile Of a Test

```
go test -p=1 -count=1 ./internal/core -short -race -timeout=100s -run=TestXXXX -memprofile mem.out
```

## Vetting

```
go vet ./...
```

## Vulnerability Checks

```
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

## Check Potentiel Nil Panics

Install https://github.com/uber-go/nilaway.

```
go install go.uber.org/nilaway/cmd/nilaway@latest
```

Run the checks:

```
nilaway ./...
```

## List Packages & Their CGo files

```
go list -f '{{with .Module}}{{.Path}}{{end}} {{.CgoFiles}}' -deps ./... | grep -v '^\s*\[\]$'
```
