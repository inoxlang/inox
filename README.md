# Inox

<img src="https://avatars.githubusercontent.com/u/122291844?s=200&v=4" alt="a shield"></img>

🛡️ Inox is a [secure](#injection-prevention) programming language that makes [scripts](#declaration-of-cli-parameters--environment-variables) and [concurrent code](#concurrency) easy to write.

## Installation

An archive with a Linux binary and some examples is available in [release assets](https://github.com/inoxlang/inox/releases), if you want to compile the language yourself go [here](#compile-from-source).

- uncompress the archive
- install the `inox` binary to `/usr/local/bin`
  ```
  sudo install inox -o root -m 0755 /usr/local/bin/inox
  ```
### Editor support

- [VSCode](https://marketplace.visualstudio.com/items?itemName=graphr00t.inox)
- [Vim](https://github.com/inoxlang/inox-vim)
- [Sublime Text](https://github.com/inoxlang/inox-sublime)

## Learning Inox

The basics of the language are explained [here](./docs/language-basics.md).\
You can also explore the `examples/` folder and start with scripts in `examples/basic`.\
To learn scripting specifically go [here](./docs/scripting-basics.md).\
View [Shell Basics](./docs/shell-basics.md) to learn how to use Inox interactively.

⚠️ The language is not production ready yet.

## Features

Security:
- [Injection Prevention](#injection-prevention)
- [Permission system](#permission-system)
  - [Required permissions](#required-permissions)
  - [Isolation of dependencies](#isolation-of-dependencies)
  - [Dropping permissions](#dropping-permissions)
- [DoS Mitigation (WIP)](#dos-mitigation)

Scripting:
- [Easy declaration of CLI Parameters](#declaration-of-cli-parameters--environment-variables)
- [Transactions & Effects (WIP)](#transactions--effects-wip)
- [Simplified resource manipulation](#simplified-resource-manipulation)

Other:
- [Concurrency](#concurrency)
  - [Coroutines (Goroutines)](#coroutines-goroutines)
  - [Lifetime jobs](#lifetime-jobs)
- [Communication](#communication)

### Injection Prevention

In Inox interpolations are always restricted in order to prevent **injections**.
When you dynamically create URLs the interpolations are restricted based on where the interpolation is located (path, query).

<img src="./docs/img/url-injection.png"></img>

<!-- code that appear in the image
> path="/data?admin=true"
> public_data=read!(https://private-service{path}?admin=false
URL expression: result of a path interpolation should not contain any of the following substrings: "..", "\" , "*", "?"'>
-->

Checked strings are strings that are validated against a pattern. When you dynamically
create a checked string all the interpolations must be explicitly **typed**.

<img src="./docs/img/query-injection.png"></img>

<!-- code that appear in the image
> id = "1 or 1=1"
> query=%sql.query`SELECT * FROM users WHERE id = {{int:id}}`
runtime check error: 0 or 1=1 does not match %sql.int
-->

### Permission system

#### **Required permissions** 

Inox features a fine-grained **permission system** that restricts what a module is allowed to do, here are a few examples of permissions:
- access to the filesystem (read, create, update, write, delete)
- access to the network (several distinct permissions)
    - HTTP (read, create, update, delete, listen)
    - Websocket (read, write, listen)
    - DNS (read)
    - Raw TCP (read, write)
- access to environment variables (read, write, delete)
- create coroutines (multi threading)
- execute specific commands

Inox modules always start with a **manifest** that describes the required permissions.

<img src="./docs/img/fs-malicious-input.png"></img>

<!-- code that appear on the image
manifest {
  permissions: {
    read: %/tmp/...
  }
}

malicious_user_input = /home/
....
print(fs.ls!(malicious_user_input))

-->

When a forbidden operation is performed the module panics with an error:\
`core: error: not allowed, missing permission: [read path(s) /home/]`

#### **Isolation of dependencies**

In imports the importing module specifies the permissions it **grants** to the imported module.

`./app.ix`

```
manifest {
  permissions: {
    read: %/...
    create: {routines: {}}
  }
}

import lib ./malicious-lib.ix {
  arguments: {}
  allow: {
    read: %/tmp/...
  }
}
```

`./malicious-lib.ix`

```
manifest {
  permissions: {
    read: %/...
  }
}

data = fs.read!(/etc/passwd)
```

If the imported module ask more permissions than granted an error is thrown:\
`import: some permissions in the imported module's manifest are not granted: [read path(s) /...] `

#### **Dropping permissions**

Sometimes programs have an **initialization** phase, for example a program reads a file or performs an HTTP request to fetch its configuration.
After this phase it no longer needs some permissions so it can drop them.

```
drop-perms {
  read: %https://**
}
```

### DoS mitigation

#### **Limitations (WIP)**

Limitations limit the **speed** at which some actions are performed, the minimum required values/rates are specified in the manifest.
This feature is still in development and will be fully implemented soon.\
Limitations will for example allow the developer to **restrict** the share of disk/network bandwidth allocated to a http request handler.

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

### Sensitive data protection (WIP)

#### **Secrets**

Secrets are special Inox values, they can only be created by defining an **environment variable** with a pattern like %secret-string.
- The content is **hidden** when printed or logged
- Secrets are not serializable, so you **cannot** send them over the network
- A comparison involving a secret always returns **false**

```
manifest {
    ...
    env: %{
        API_KEY: %secret-string
    }
    ...
}

API_KEY = env.initial.API_KEY
```


#### **Visibility (WIP)**

TODO: explain


### Declaration of CLI parameters & environment variables

CLI parameters & environment variables can be described in the manifest:

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
        clean-existing: {
            pattern: %bool
            default: false
            description: "if true delete <dir> if it already exists"
        }
    }
    env: {
      API_KEY: %secret-string
    }

    permissions: {
        write: IWD_PREFIX # initial working directory
        delete: IWD_PREFIX
    }
}

# {
#   "dir": ...
#   "clean-existing": ...
# }
args = mod-args

API_KEY = env.initial.API_KEY
```

#### Help message generation

```
$ inox run test.ix 
not enough CLI arguments
usage: <dir path> [--clean-existing]

required:

  dir: %path
      root directory of the project

options:

  clean-existing (--clean-existing): boolean
      if true delete <dir> if it already exists
```

### Transactions & Effects (WIP)

Inox allows you to attach a **transaction** to the current execution context (think SQL transactions).
When a **side effect** happens it is recorded in the transaction. If the execution is cancelled for whatever reason the transaction is automatically **rollbacked** and 'reversible' effects are reversed. (A 'mode' that causes side effects to only be committed if the transaction succeed is also planned)

```
tx = start_tx()

# effect
fs.mkfile ./file.txt 

# rollback transaction --> delete ./file.txt
cancel_exec() 
```

#### **Simplified resource manipulation**

- The builtin [**read**](./docs/shell-basics.md#read) function can read directories / files / HTTP resources and parse their content.
```
read ./dir/
read ./file.json  # parsed by default
read https://jsonplaceholder.typicode.com/posts  # parsed by default
```

- The builtin [**create**](./docs/shell-basics.md#create) function can create directories / files / HTTP resources.
```
create ./dir/
create ./file.txt "hello world !"
create https://example.com/posts tojson({title: "hello"})
```

Learn more [here](./docs/shell-basics.md#resource-manipulation)

### Concurrency

#### **Coroutines (Goroutines)**

```
coroutine = go {globals: .{print}} do {
  print("hello from goroutine !")
  return 1
}

# 1
result = coroutine.wait_result!()
```

#### **Coroutine Groups**

```
group = RoutineGroup()
coroutine1 = go {group: group} do read!(https://jsonplaceholder.typicode.com/posts/1)
coroutine2 = go {group: group} do read!(https://jsonplaceholder.typicode.com/posts/2)

files = group.wait_results!()
```

#### **Lifetime jobs**

Lifetime jobs are coroutines linked to an object.

```
object = {
  lifetimejob #handle-messages {
    for msg in watch_received_messages(self){
      # handle messages
    }
  }
}
```

### Communication

In Inox objects can communicate in several different ways.

#### **Message sending**

The `sendval` construct allows an object to send message to another object.

```
object = {
  inner: {
    method: fn(){
      # supersys is a keyword that designates the parent object here 
      sendval "hello" to supersys
    }
  }
  
  lifetimejob #handle-messages {
    for msg in watch_received_messages(self){
      print(msg) # "hello"
    }
   }
}
```

TODO: explain


## Compile from source

- clone this repository
- `cd` into the directory
- run `go build cmd/inox/inox.go`



