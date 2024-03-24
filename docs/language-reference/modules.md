[Table of contents](./README.md)

---

# Modules

- [Module Parameters](#module-parameters)
- [Preparation, checking & execution phases](#preparation-checking-and-execution-phases)
- [Inclusion Imports](#inclusion-imports)
- [Module Imports](#module-imports)
- [Permissions](./permissions.md)
- [Limits](./limits.md)
- [Host Definitions](#host-definitions)
- [Databases Definitions](./databases.md)
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

## Preparation, Checking And Execution Phases

The execution of a module has several phases:

**preparation phases**:

- **Parsing**
- [Pre-initialization](./pre-initialization.md)
- **Opening of Databases**
- [Static Check](./static-check.md)
- [Symbolic Evaluation/Check](./symbolic-evaluation.md)

**actual execution phases**:

- [Compilation](./compilation.md) (if using the [bytecode interpreter](./evaluation.md))
- [Evaluation](./evaluation.md)

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

Inclusion imports include the content of a file in the current file. Includable files are
useful to decompose a module or regroup pattern definitions/functions shared
between modules.

```
# main.ix
manifest {}

import ./patterns.ix

# patterns.ix
includable-file

pattern user = {
    name: str
    profile-picture: url
}
```

⚠️ Includable files can only contain definitions (functions, patterns, ...).

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

## Host Definitions

**WORK IN PROGRESS**

The `host-definitions` section of the manifest defines **Inox hosts**.

```
manifest {
    host-resolution: :{
        ldb://main : /mydb
    }
}
```

Host definitions are inherited by descendant modules.
Database definitions implicitly define hosts.

## Main Module

In Inoxlang "a" **main module** does not always refer to the first module being
executed because in some cases modules can invoke other "main" modules. In
general the main module is the "main" module of "a" project.

[Back to top](#modules)
