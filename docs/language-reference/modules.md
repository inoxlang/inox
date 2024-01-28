[Table of contents](./language.md)

---

# Modules

- [Module Parameters](#module-parameters)
- [Permissions](#permissions)
- [Execution Phases](#execution-phases)
- [Inclusion Imports](#inclusion-imports)
- [Module Imports](#module-imports)
- [Limits](#limits)
- [Main Module](#main-module)

An Inox **file module** is a code file that starts with a manifest.

There are several kinds of **file** modules:

- `unspecified`: the default
- `spec`: modules with a filename ending with `.spec.ix`
- `application`: modules with `kind: "application"` in their manifest

There are also **embedded** module kinds:

- `userlthread`: lthread modules created with spawn expressions such as
  `go do { }`
- `testsuite`
- `testcase`
- `lifetimejob`

[Issue discussing new module kinds](https://github.com/inoxlang/inox/issues/38).

Each module runs in a dedicated Golang **goroutine**.

## Module Parameters

Module can take parameters, for the main module they correpond to the CLI
parameters.\
In the following module manifest two parameters are defined: **dir** and
**verbose**:

```
manifest {
    parameters: {
        # positional parameters are listed at the start
        {
            name: #dir
            pattern: %path
            rest: false
            description: "root directory of the project"
        }
        # non positional parameters
        verbose: %bool
    }
}

dir = mod-args.dir
clean-existing = mod-args.clean-existing
```

Arguments should be added after the path when executing the program:

```
inox run [...run options...] ./script.ix ./dir/ --verbose
```

## Permissions

The permissions section of the manifest lists the permissions required by the
module. Permissions represent a type of action a module is allowed (or
forbidden) to do. Most IO operations (filesystem access, HTTP requests) and
resource intensive operations (lthread creation) necessitate a permission.

**Examples:**

```
# reading any file in /home/user/ or below
manifest {
    permissions: {
        read: {
            %/home/user/...
        }
    }
}

# sending HTTP GET & POST requests to any HTTPS server
manifest {
    permissions: {
        read: {
            %https://**
        }
        write: {
            %https://**
        }
    }
}

# creating an HTTPS server listening on localhost:8080
manifest {
    permissions: {
        provide: https://localhost:808
    }
}

# reading from & writing to the database ldb://main
manifest {
    permissions: {
        read: {
            ldb://main
        }
        write: {
            ldb://main
        }
    }
}

# creating lightweight threads
manifest {
    permissions: {
        create: {
            threads: {}
        }
    }
}
```

## Execution Phases

The execution of a module has several phases:

**preparation phases**:

- **Parsing**
- [Pre-initialization](#pre-initialization)
- **Opening of Databases**
- [Static Check](#static-check)
- [Symbolic Evaluation/Check](#symbolic-evaluation)

**actual execution phases**:

- [Compilation](#compilation) (if using the [bytecode interpreter](#evaluation))
- [Evaluation](#evaluation)

## Result

Inox modules can return a value with a return statement:

```
# return-1.ix
manifest {}

return 1
```

This feature is generally used by imported modules to return a result or export
functions.

## Inclusion Imports

Inclusion imports include the content of a file in the current file. They are
useful to decompose a module or regroup pattern definitions/functions shared
between modules.

```
# main.ix
manifest {}

import ./patterns.ix

# patterns.ix
includable-chunk

pattern user = {
    name: str
    profile-picture: url
}
```

⚠️ This feature is currently in development ! File inclusion will follow strict
rules.

## Module Imports

As the name imply this language construct imports a **module**: an Inox file
that starts with a manifest. Here is a minimal example:

```
# main.ix
manifest {
    permissions: {
        read: %/...    # don't forget the read permission
    }
}

import result ./return_1.ix {}

print(result) 


# return-1.ix
manifest {}

return 1
```

Module imports starts by the creation of a new instance of the imported module.
Then the instance is executed and its result is returned.

⚠️ As a consequence, if you import the same module from two different files, the
instances will not be the same. Let's see an example.

We have the modules `/main.ix`, `/lib1.ix`, `/lib2.ix`.

- Both `main` and `lib1` import `lib2`
- `main` also imports `lib1`

```
# --- main.ix ---

manifest {
    permissions: {
        read: %/...
    }
}

import lib1 ./lib1.ix {}

# this instance of lib2 is not the same as the one in /lib1.ix.
import lib2 ./lib2.ix {}


# --- lib1.ix ---

manifest {
    permissions: {
        read: %/...
    }
}

# this instance of lib2 is not the same as the one in /main.ix.
import lib2 ./lib2.ix {}


# --- lib2.ix ---

return {
    state: {
        # ....
    }
}
```

### Arguments

As explained [here](#module-parameters) module can take parameters. When an
imported module does have parameters you have to pass arguments to it.

```
# main.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

import result ./add.ix {
    args: {1, 2}
} 

print(result) 

# add.ix
manifest {
    parameters: {
        {
            name: #first_operand
            pattern: %int
        }
        {
            name: #second_operand
            pattern: %int
        }
    }
}

return (mod-args.first_operand + mod-args.second_operand)
```

### Granting Permissions

In most cases the modules you import will require access to the filesystem or
the network. You can grant them the required permissions in the **allow**
section of the import.

> Note: in the following example IWD_PREFIX refers to a prefix path pattern
> matching the working directory

```
# main.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

import read-config ./read-config.ix {
    allow: {read: IWD_PREFIX}
}

config = read-config()
# ...


# read-config.ix
manifest {
    permissions: {
        read: IWD_PREFIX
    }
}

return fn(){
    # ...
}
```

⁉️ So I need to write a manifest + specify permissions in **EACH** file ?\
-> No, you will typically use [inclusion imports](#inclusion-imports) for
trusted, local files. Modules are useful to provide a library or to decompose an
application in smaller parts.

## Limits

Limits limit intensive operations, there are three kinds of limits:
**[byte rate](#byte-rate-limits)**, **[frequency](#frequency-limits)** &
**[total](#total-limits)**. Limits are defined in module manifests.

```
manifest {
    permissions: {
        ...
    }
    limits: {
        "fs/read": 10MB/s
        "http/req": 10x/s
    }
}
```

### Sharing

At runtime a counter will be created for each limit, the behaviour of the
counter is specific to the limit's kind. Limits defined by a module will be
shared with all of its child modules/threads. In other words when the module
defining the limit or one if its children performs an operation a shared counter
is decremented.

**Example 1 - CPU Time**

```
# ./lib.ix
manifest {}

do_intensive_operation1()
...
return ...


# ./main.ix
manifest {
    limits: {
        "execution/cpu-time": 1s
    }
}

# all CPU time spent by the lib is added to the counter of ./main.ix
import lib ./lib.ix {} 

# all CPU time spent by the child threads are added to the counter of ./main.ix
lthread = go do {
    do_intensive_operation2()
}

...
```

[Issues with the CPU time limit.](https://github.com/inoxlang/inox/issues/19)

**Example 2 - Simultaneous Thread Count**

```
# ./main.ix
manifest {
    limits: {
        "threads/simul-instances": 2
    }
}

# lthread creation, the counter is decreased by one
lthread = go do {
    # lthread creation inside the child lthread, the counter is decreased by one
    go do {
        sleep 1s
    }
    sleep 1s
}

# at this point 2 lthreads are running, attempting to create a new one would cause an error.
...
```

### Byte Rate Limits

This kind of limit represents a number of bytes per second.\
Examples:

- `fs/read`
- `fs/write`

### Frequency Limits

This kind of limit represents a number of operations per second.\
Examples:

- `fs/create-file`
- `http/request`
- `object-storage/request`

### Total Limits

This kind of limit represents a total number of operations or resources.
Attempting to make an operation while the counter associated with the limit is
at zero will cause a panic.\
Examples:

- `fs/total-new-files` - the counter can only go down.
- `ws/simul-connections` - simultaneous number of WebSocket connections, the
  counter can go up & down since connections can be closed.
- `execution/cpu-time` - the counter decrements on its own, it pauses when an IO
  operation is being performed.
- `execution/total-time` - the counter decrements on its own.

## Main Module

In Inoxlang "a" **main module** does not always refer to the first module being
executed because in some cases modules can invoke other "main" modules. In
general the main module is the "main" module of "a" project.

[Back to top](#)
