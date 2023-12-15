[Install Inox](../README.md#installation) | [Language Reference](./language-reference.md) |  [Built-in Functions](./builtins.md) | [Project](./project.md) | [Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

<details> 
<summary> Tip: If you are reading on GitHub, you can display an outline on the side. </summary>

![image](https://github.com/inoxlang/inox/assets/113632189/c4e90b46-eb9c-4a0f-84ad-3389d2c753d4) 
</details>

-----

# Web Application Development

> This tutorial is not finished yet.

In this tutorial you will learn how to create a basic **todo** application.
Before going further create a new empty **todo** [project](./project.md) using Visual Studio Code and open the workspace.

[1) HTTP Server](#1-http-server)\
[2) Filesystem Routing](#2-filesystem-routing)\
[3) Database](#3-database)


## 1) HTTP Server

In the **Project Filesystem** (virtual) let's create the following folders:
- **/static** - this folder will contain the static files of the application (JS, CSS, images)
- **/routes** - this folder will contain the handler files for dynamic content

Now let's create **/main.ix** file with the following content:

```inox
const (
    HOST = https://localhost:8080
)

manifest {
    permissions: {
        provide: HOST
        read: %/...
        write: %/.dev/self_signed* # permission to persist the generated self-signed certificate.
    }
}

server = http.Server!(HOST, {
    routing: {
        static: /static/
        dynamic: /routes/
    }
})

server.wait_closed()
```

ℹ️ The `provide: HOST` permission is required by the server and
the `read %/...` permission is required by the filesystem routing.

## 2) Filesystem Routing

When the server receives a request it determinates what is the handler module (file)
for the request and invokes the handler. The routing rules are the following:

| Request's path | HTTP method | Possible handler paths |
| ----------- | ----------- | ----------- |
| / | GET | /GET-index.ix , /index.ix |
| /about | GET | /GET-about.ix , /about.ix , /about/GET.ix , /about/index.ix |
| /users | POST | /POST-users.ix , /users.ix , /users/POST.ix , /POST/users/index.ix |

Now create the `/routes/index.ix` file to handle the home page:
```inox
manifest {
    parameters: {}
}

return html<html>
<head>
    <meta charset="utf-8"/>
    <title>ToDo App</title>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
</head>
<body>
    Welcome to ToDo App !
</body>
</html>
```

ℹ️ Since the manifest does not contain any parameter the only accepted methods are **GET** and **HEAD**.

## 3) Database

### 3.1) Database Creation

We need a database in order to store & retrieve Todo items, let's define a database in **/main.ix**:
```

# the preinit block is executed before the manifest.
preinit {
    import ./schema.ix
}

manifest {
    permissions: {
        provide: HOST
        read: %/...
        write: {
            %/.dev/self_signed*
            %/databases/main/...    # required by the database
        }
    }
    databases: {
        main: {
            resource: ldb://main

            # location of the data in the project filesystem
            resolution-data: /databases/main/   

            expected-scheme-update: true
        }
    }
}


dbs.main.update_schema(%db-schema, {
    inclusions: :{
        %/users: [] # initialize the .users Set
    }
})

print("database schema: ", dbs.main.schema)


[...HTTP server creation...]
```

Create a `/schema.ix` file in which we define the database schema:

```
pattern todo = {
    title: str
    done: bool
}

pattern user = {
    name: str
    todos: []todo
}

pattern db-schema = {
    users: Set(user, #url)
}
```

Execute the code a single time then remove the `update_schema(...)` call and `expected-scheme-update: true` from the manifest.

You should now be able to access the set of users by typing: `dbs.main.users` !

### 3.2) Access From Other Modules

The database we created can be accessed by `/main.ix` but not by other modules. 
Let's fix that by modifying the manifest of `/routes/main.ix`:

```
manifest [
    # allow the module to access the main database.
    databases: /main.ix
    permissions: {
        read: {
            ldb://main
        }
        # you can also add the write permission if needed.
    }
}
```

`dbs.main` should now be accessible from inside the module.