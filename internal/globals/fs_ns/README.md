# Filesystem Namespace

This package implements the functions provided by the ``fs`` namespace (available by default in the global scope).
- [make.go](./make.go)
    - ``mkdir``
    - ``mkfile``
    - ``cp`` (Copy)

- [update.go](./update.go)
    - ``rename``
    - ``rm``
    - ...

- [read.go](./read.go)
    - ``read``
    - ``read_file``
    - ``find``
    - ``isdir``
    - ...

- [find.go](./find.go)
    - ``find``

- [globbing.go](./globbing.go)
    - ``glob``

- [open.go](./open.go)
    - ``open``

This package also contains several implementations of [afs.Filesystem](../../afs/abstract_fs.go):
- [MemFilesystem](./memory_filesystem.go)
- [MetaFilesystem](./meta_filesystem.go)
- [OsFilesystem](./os_filesystem_unix.go)

File watching is implemented in [watcher_unix.go](./watcher_unix.go).