[Install Inox](../README.md#installation) | [Language Basics](./language-basics.md) |  [Built-in Functions](./builtin.md) | [Project](./project.md) | [Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

-----


# Web Application Development

In this tutorial you will learn how to create a basic a **todo** application.
Before going further create a new empty **todo** [project] using Visual Studio Code and open the workspace.

Outline:\
[HTTP Server](#1-http-server)

## 1) HTTP Server

In the **Project Filesystem** (virtual) let's create the following folders:
- **/static** - this folder will contain the static files of the application (JS, CSS, images)
- **/routes** - this folder will contain the handler files for dynamic content

Now let's create **main.ix** file with the following content:

```inox
const (
    HOST = https://localhost:8080
)

manifest {
    permissions: {
        provide: HOST
        read: %/...
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

| Request's path | Request's method | Possible handler paths |
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