[Install Inox](../README.md#installation) | [Language Basics](./language-basics.md) | [Scripting Basics](./scripting-basics.md) | [Built-in Functions](./builtin.md)

-----

# Inox Shell (REPL)

Launch the shell with the following subcommand:
```
inox
```

Note: the `inox shell` command can also be used.

Before starting the shell ``inox`` will execute the startup script `~/.config/inox/startup.ix` (or $XDG_CONFIG_HOME/inox/startup.ix) that set the [configuration](#configuration).

- [Pseudo commands](#pseudo-commands-quit-clear)
- [Syntax](#syntax)
  - [Calling a function](#calling-a-function)
  - [Variables](#variables)
  - [Pipe statements](#pipe-statements)
- [Type checker](#type-checker)
- [Execute Inox scripts](#execute-inox-scripts-from-the-repl)
- [Execute commands](#execute-commands)
- [Resource manipulation](#resource-manipulation)
  - [Read](#read)
    - [Directory](#directory)
    - [File](#file)
    - [HTTP](#http-resource)
    - [Raw](#raw-data)
  - [Create](#create)
  - [Update](#update)
  - [Delete](#update)
  - [Find](#find)
- [Shell configuration](#configuration)

## Pseudo Commands (quit, clear)

- `quit` pseudo command stops the process.
- `clear` pseudo command clears the screen.

## Syntax

### Calling a Function

Copy the following code in the Inox shell & press Enter:
```
fn f(...args){ print $args } # this function prints its arguments
```

Functions in Inox can be called in several ways:
```
f() 
output: []

f;
output: []

f 1 2 3
output: [1, 2, 3]
```

The last call syntax is named a **command-like** call.

### Variables

When you reference a variable in the shell (or in an Inox script) you can directly use its name
or prefix it with a dollar (two dollars for globals, but you will rarely use them this way).

```
a = 1; b = 2
(a == $b)
```

⚠️ In a **command-like** call `a` is considered as an identifier value that can also be written `#a`, 
you have to use `$a` to reference a.

```
f $a a
output: [1, #a]
```

### Pipe Statements

Pipe statements are analogous to pipes in Unix but they act on the values returned by functions, not 
file descriptors.

Here is an example:

```
map [{value: "a"}, {value: 1}] .value | filter $ %int
```

- in the first call we extract the .value property of several objects using the `map` function
- in the second call we filter the result of the previous call
  - `$` is an anonymous variable that contains the result of the previous call
  - `%int` is a pattern matching integers


Pipe expressions allows you to store the final result in a variable:
```
ints = | map [{value: "a"}, {value: 1}] .value | filter $ %int
```

### Help

```
help <name of function>

example:
help find
```

## Type Checker

The type checker performs various checks before the input code is executed, allowing you to quickly catch errors.

```
> map 1 .name

check(symbolic): shell-input:1:5: : invalid value for argument at position 0: type is %int, but %iterable was expected
```

This is convenient, but there are many cases where you **don't** want such strictness !
Let's say you are executing the following command:

```
read https://jsonplaceholder.typicode.com/posts | map $ .title
```

The type checker will complain that `$` is not an %iterable, that's pretty annoying.
You can postpone the type check of this argument at runtime by prefixing it with '~'.

```
read https://jsonplaceholder.typicode.com/posts | map ~$ .title
```

Note: '~' can be added in front of any expresion that is an argument in a call.


## Execute Inox Scripts from the REPL

```
run ./myscript.ix
```

⚠️ Paths always start with `./, ../ or /` , if you type `run myscript.ix` it won't work.\
⚠️ The script will be potentially granted all the permissions of the shell !

## Execute Commands

```
ex echo "hello"   # 'ex echo hello' will not work
ex #go help
```

NOTE: Almost no commands are allowed by default, edit your startup script in `.config/inox` to allow more commands (and subcommands).

## Resource Manipulation

From now on we will references files, HTTP servers and endpoints as "resources".
You can easily manipulate resources using ``read | create | update | delete | provide`` followed by the resource's name.

### Read

Read is a powerful function that allows you to get the content of files, directories & HTTP resources.

#### **Directory**

Reading the entries of a directory ``read ./dir/`` returns a list of %file-info:
```
[
    dir/
    file.txt 1kB 
]
```

#### **File**

By default the `read` function parses the content of the read file, the extension
is used to determinate the type of content.

- Reading a text file returns a string: ``read ./file.txt` ->``"hello"`
- Reading a JSON file returns Inox values (objects, lists, ...) resulting from the parsing : 
    ``read ./file.json``
    ```json
    {"key": "value"}
    ```

#### **HTTP Resource**

By default the `read` function parses the content of the read resource, the Content-Type header 
is used to determinated the type of content.

Reading an JSON HTTP resource: 
``read https://jsonplaceholder.typicode.com/posts/1``

```json
{
  "body": "quia et suscipit\nsuscipit recusandae consequuntur expedita....", 
  "id": 1.0, 
  "title": "sunt aut facere repellat provident occaecati excepturi optio reprehenderit", 
  "userId": 1.0
}
```

Reading a HTML resource will return a `%html.node`.

#### **Raw Data**

You can disable parsing by adding the `--raw` switch **after** the resource name, a byte slice (%bytes)
will be returned instead.

### Create

Create a dir: ``create ./dir/``

Create a file: ``create ./file.txt [optional string content]``

### Update

Append to a file: ``update ./file.txt append <string>``

Patch an HTTP resource: ``update <url> <string | object>``

### Delete

Use ``delete <resource>`` for deletion. The deletion is recursive for folders.

### Find

Recursivelly find all JSON files in a directory.
```
find %./**/*.json ./
```

**You can read more about the built-in functions [here](./builtin.md)**

## Configuration

The startup script `~/.config/inox/startup.ix` (or $XDG_CONFIG_HOME/inox/startup.ix) is executed before the shell is started,
it returns the configuration for the shell. You can modify this file if you need more permissions.

The startup script can be specificied using the `-c` flag:
```
inox shell -c startup.ix
```

### Minimal

The default startup.ix gives at lot of permissions to the shell, if you want to highly restrict
what the shell is allowed to do you should start with a minimal startup script & only add required permissions.

The smallest possible startup script returns an empty object:
```
manifest {}

return {}
```

The permissions required by the startup script are also granted to the shell: here the manifest is empty,
so the shell will not be able to read files or make network requests. 

Let's add the prompt configuration and a few permissions to make our shell usable.

```
const (
  HOME_PREFIX = /home/user/... # replace with your HOME directory
  TMP = /tmp/...
)

manifest {
  # allow the shell to read|write|delete any file in /tmp/... or in the user's HOME directory (or below).
  permissions: {
    read: { HOME_PREFIX, TMP }
    write: { HOME_PREFIX, TMP }
    delete: { HOME_PREFIX, TMP }
  }
}

return {
  builtin-commands: [#cd, #pwd, #whoami, #hostname]
  trusted-commands: [#echo, #less, #grep, #cat]

  prompt: [
    [@(whoami())  #bright-black #black]
    ["@" #bright-black #black]
    [@(hostname())  #bright-black #black]
    ":"
    [@(pwd())  #bright-blue #blue]
    "> "
  ]
}
```

### Builtin Commands

Builtin commands are provided by the shell, note that commands do not exist natively in Inox,
typing `cd ./dir/` is equivalent to `cd(./dir/)` because **cd** is a function.

There are only a few builtin commands availabe:
- cd
- pwd
- whoami
- hostname

### Trusted Commands

Trusted commands are commands that are fully trusted, when you execute a trusted command the standard input is redirected
to the spawned process.

You can try that with the following command 
```
less <file>
```

*press 'q' to leave, as you would do when using **less** in bash*

### Prompt

Here is an example of prompt configuration:
```
[
  [@(whoami())  #bright-black #black]
  ["@" #bright-black #black]
  [@(hostname())  #bright-black #black]
  ":"
  [@(pwd())  #bright-blue #blue]
  "> "
]
```

- each part is described by a value or a list, allowed values are:
  - strings & values similar to strings (paths, urls, ...) 
  - lazy expressions
- lists describe the part followed by 2 colors
  - the first color is used when the terminal's background is darkish
  - the second color is used when the terminal's background is light
- lazy expressions such as @(whoami()) are evaluated each time the prompt is printed
  - they must be calls
  - only the whoami, hostname & pwd functions are allowed
