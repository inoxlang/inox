# Filesystem Namespace

This package implements the functions provided by the ``fs`` namespace (available by default in the global scope).
- [make.go](./make.go)
    - ``mkdir``
    - ``mkfile``
    - ``cp`` (Copy)

- [update.go](./update.go)
    - ``rename`` (**mv** is an alias)
    - ``rm`` (**remove** is an alias)
    - ...

- [read.go](./read.go)
    - ``read``
    - ``read_file``
    - ``ls``
    - ``isdir``
    - ``isfile``
    - ``exists``
    - ``get_tree_data``

- [find.go](./find.go)
    - ``find``

- [globbing.go](./globbing.go)
    - ``glob``

- [open.go](./file.go)
    - ``open``

This package also contains several implementations of [afs.Filesystem](../../afs/abstract_fs.go):
- [MemFilesystem](./memory_filesystem.go)
- [MetaFilesystem](./meta_filesystem.go)
- [OsFilesystem](./os_filesystem_unix.go)

File watching is implemented in [watcher_unix.go](./watcher_unix.go).