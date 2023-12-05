[Install Inox](../README.md#installation) |
[Language Reference](./language-reference.md) |
[Shell Basics](./shell-basics.md) | [Built-in Functions](./builtin.md) |
[Web App Development](./web-app-development.md) |
[Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

# Inox Projects

## Editor

Vscode is currently the only IDE/editor that supports Inox using the
[Inox extension](https://marketplace.visualstudio.com/items?itemName=graphr00t.inox).
The extension is required to work on Inox projects.

## Starting the Project Server

### Locally

If your OS is Linux or if you use WSL you can start the **project server** with
the following command:

```
inox project-server
```

This server is listening on `localhost:8305` by default. The listening port can
be changed with the **-config** flag: `-config='{"port":8305}'`.

Browser automation can be allowed by adding the `--allow-browser-automation` switch.

**You can install the [inox daemon](./inox-daemon.md) to start the project server automatically.**

### Remote Linux Server

Install the [inox daemon](./inox-daemon.md) in order to start the project server automatically and to expose it.

## Creating a Project

On you development machine create a `<myproject>` folder for the project. Open
the folder with Visual Studio Code, and execute the following command
`Inox: Initialize new Project in Current Folder`.

Open the generated .code-workspace file and click on **Open Workspace**.

## Project Secrets

Project Secrets are **persisted** secrets, they can be created, updated &
deleted from the **Inox Project** tab in VScode.

### Retrieving project secrets

The global variable `project-secrets` is a global variable containing the
secrets, it is only available from the main module.\
If you defined a secret named `MY_SECRET` you can retrieve it with the following
code:

```
secret = project-secrets.MY_SECRET
```
