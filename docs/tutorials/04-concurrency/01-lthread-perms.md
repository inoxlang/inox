# Permissions 

```
manifest {
    permissions: {
        create: {threads: {}}
        read: %/...
    }
}

# Lthreads created by spawn expressions inherit almost all of the permissions of
# the parent module by default. The thread creation permission is removed.
# You can specify what permissions are granted in the allow section of the meta value.

# create a lthread with no permissions.
thread1 = go {
    allow: {}
} do {
    # A read permission is missing.
    # If an error is shown by the debugger on the following line, press the arrow to continue.
    return fs.ls!(/)
}

assign result err = thread1.wait_result()
print("err:", err)

# Create a lthread allowed to read any file and directory.
thread2 = go {
    allow: { read: %/... }
} do {
    return fs.ls!(/)
}

print("entries:", thread2.wait_result!())
```