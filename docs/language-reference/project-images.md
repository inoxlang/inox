[Table of contents](./README.md)

---

# Project Images

_This feature is work in progress._

A **project image** contains a filesystem snapshot and some metadata about the project.
It includes all files and folders except the ones with a dot name (e.g. `.dev/`, `.file`).

Project images will ultimately be exportable as **zip** archives with the following layout.  

```
fs/ ---- files
    main.ix
    schema.ix
    routes/
    static/
    ...
inox-image.json
```

## Testing

Project images are used during [program testing](./testing.md#program-testing).