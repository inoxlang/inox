[Install Inox](../README.md#installation) | [Language Basics](./language-basics.md) | [Shell Basics](./shell-basics.md) | [Built-in Functions](./builtin.md)

-----

# Scripting Basics

In this tutorial you will learn how to write Inox scripts & use the most important functions.

## Hello world

```
manifest {}

print "hello world !"
```

An Inox program always starts with a manifest, the manifest lists:
- the [permisssions](./features/permissions.md) required by the program
- the [parameters](#parameters) of the program

You will learn about the manifest throughout this tutorial.

Run the script using the following command:
```
inox run script.ix
```

### Shebang

Inox scripts support shebangs
- add `#!/usr/local/bin/inox run` at the top of the file
- `chmod u+x script.ix`
- ./script.ix

## A Script to Generate a Project Directory Structure 

## Version 1

Our `gen-project.ix` script will have 2 parameters:
- the location of the directory that will be created, it's a positional parameter (it is not prefixd with --).
- a `--clean-existing` switch to delete the directory if it already exists.

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
        clean-existing: %bool
    }
}
```

The script needs the `write` permission to create the directory structure and the `delete` permission
to remove the directory when we use the `--clean-existing` switch.\
Let's add the `permissions` section in the manifest

```
    permissions: {
        write: IWD_PREFIX # initial working directory
        delete: IWD_PREFIX
    }
```

Now let's write the code for the program.
First we need to get the module's argument.

```
dir = mod-args.dir
clean-existing = mod-args.clean-existing
```

If `clean-existing` is true we have to recursively remove the directory,
we can easily achieve this by using `fs.remove` or `delete`

```
if clean-existing {
    fs.remove $dir

    # this can also be written as:
    fs.remove(dir)
}
```

Now let's create a basic directory structure, we can use `fs.mkdir` with
a [dictionary](./language-basics.md#dictionaries) literal (dictionary literals start with `:{`).

```
project_name = dir.name 
readme_content = concat "# " project_name

fs.mkdir $dir :{
    ./README.md: readme_content
    ./.env: "" # empty
    ./src/: [./app.c]
}
```

Here is the complete code for our script. 

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
        clean-existing: %bool
    }

    permissions: {
        write: IWD_PREFIX # initial working directory
        delete: IWD_PREFIX
    }
}

dir = mod-args.dir
clean-existing = mod-args.clean-existing

if clean-existing {
    fs.remove $dir
}

project_name = dir.name
readme_content = concat "# " project_name

fs.mkdir $dir :{
    ./README.md: readme_content
    ./.env: "" # empty
    ./src/: [./app.c]
}
```

Let's run the script:
```
inox run gen-project.ix ./myapp/ --clean-existing
```

## Version 2 - Verbose Switch

We want to add a verbose mode, in this mode the script will tell us if it has
deleted the target directory in the case we used `--clean-existing`.

Let's add the `verbose` parameter in the parameters section of the manifest.

```
parameters: {
    {
        name: #dir
        pattern: %path
        rest: false
        description: "root directory of the project"
    }
    # non positional parameters
    clean-existing: %bool
    verbose: {
        pattern: %bool  # mandatory
        default: false  # mandatory if the default value is not inferrable
        description: "if true the script will output more information"
    }
}
```

Note: in the parameters `clean-existing: %bool` is equivalent to:
```
clean-existing: {
    pattern: %bool
}
```

Now we add the conditional print before the call to fs.remove:

```
if clean-existing {
    if (mod-args.verbose and fs.exists(dir)) {
        print "remove" $dir
    }
    fs.remove $dir
}
```

Since we are reading the filesystem we need to add `read: IWD_PREFIX` in the permissions.