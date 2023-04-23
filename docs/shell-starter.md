# REPL

Launch the shell with the ``shell`` subcommand:
```
inox shell
```

Before starting the shell ``inox`` will execute the startup script found in `.config/inox` (or XDG_CONFIG_HOME) and grant the required permissions by the script to the shell.\
No additional permissions will be granted. You can modify the startup script in `.config/inox` if you need more permissions.

## Execute Inox scripts from the REPL

```
run ./myscript.ix
```

⚠️ Paths always start with `./, ../ or /` , if you type `run myscript.ix` it won't work.\
⚠️ The script will be potentially granted all the permissions of the shell !

## Execute commands

```
ex echo "hello"   # 'ex echo hello' will not work
ex go help
```

NOTE: Almost no commands are allowed by default, edit your startup script in `.config/inox` to allow more commands (and subcommands).


## Read, Create, Update, Delete, Provide resources

From now on we will references files, HTTP servers and endpoints as "resources".

You can easily manipulate resources using ``read | create | update | delete | provide`` followed by the resource's name.


## Read

Read the entries of a folder: ``read ./dir/``

Read a file: ``read ./file.txt``

Read an HTTP resource with: ``read https://jsonplaceholder.typicode.com/posts/1``

## Create

Create a dir: ``create ./dir/``

Create a file: ``create ./file.txt [optional string content]``

## Update

Append to a file: ``update ./file.txt append <string>``

Patch an HTTP resource: ``update <url> <string | object>``

## Delete

Use ``delete <resource>`` for deletion. The deletion is recursive for folders.

## Help for a function

```
help <name of function>

example:
help find
```

## Finding

Recursivelly find all JSON files in a directory.
```
find %./**/*.json ./
```

