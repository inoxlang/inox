# Contribution


## Tests

Run all tests with:
```
go test -race -count=1 -p=1 ./...
```

If you have Chrome installed you can set the env var RUN_BROWSER_AUTOMATION_EXAMPLES to "true". 

## Vetting

```
go vet ./...
```

## List Packages & Their CGo files
```
go list -f '{{with .Module}}{{.Path}}{{end}} {{.CgoFiles}}' -deps ./... | grep -v '^\s*\[\]$'
```