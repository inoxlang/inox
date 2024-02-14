# Virtual Filesystem

In project mode, Inox applications run inside a virtual filesystem named a **meta filesystem** that persists data on disk.
Files in this filesystem are regular files, (most) metadata and directory structure are stored in a single file named `metadata.kv`.
It's impossible for applications running inside this filesystem to access an arbitrary file on the disk.

Inox also supports in-memory filesystems.

```mermaid
graph LR

subgraph InoxBinary[Inox Binary]
  Runtime1 --> MetaFS(Meta Filesystem)
  Runtime2 --> InMemFS(In-Memory Filesystem)
  Runtime3 --> OsFS(OS Filesystem)
end

MetaFS -.-> MetadataKV
MetaFS -.-> DFile1
MetaFS -.-> DFile2
OsFS -.-> Disk


subgraph Disk

  subgraph SingleFolder[Single Folder]
    MetadataKV("metadata.kv")
    DFile1("File 01HG3BE...")
    DFile2("File 01HFHVY...")
  end
end
```

