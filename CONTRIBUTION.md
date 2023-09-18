# Contribution


## Tests

Run all tests with:
```
go test -race -count=1 -p=1 ./...
```

If you have Chrome installed you can set the env var RUN_BROWSER_AUTOMATION_EXAMPLES to "true". 

If you have a S3 bucket with read & write access you can the set the env variables read in the following [file](internal/globals/s3_ns/fs_test.go).

If you have a Cloudflare Account you can the set the env variables read in the following [file](internal/project/secrets_test.go).


# Save Memory Profile Of a Test

```
go test -p=1 -count=1 ./internal/core -short -race -timeout=100s -run=TestXXXX -memprofile mem.out
```

## Vetting

```
go vet ./...
```

## List Packages & Their CGo files
```
go list -f '{{with .Module}}{{.Path}}{{end}} {{.CgoFiles}}' -deps ./... | grep -v '^\s*\[\]$'
```
