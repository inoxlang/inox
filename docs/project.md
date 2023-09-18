[Install Inox](../README.md#installation) | [Language Reference](./language-reference.md) | [Shell Basics](./shell-basics.md) | [Built-in Functions](./builtin.md) | [Web App Development](./web-app-development.md) | [Shell Basics](./shell-basics.md) | [Scripting Basics](./scripting-basics.md)

# Inox Projects

## Editor

Vscode is currently the only IDE/editor that supports Inox using the [Inox extension](https://marketplace.visualstudio.com/items?itemName=graphr00t.inox). The extension is required to work on Inox projects.


## Starting the Project Server

Once you have installed Inox locally or on a server start the **project server** with the following command:
```
inox project-server
```

The listening port can be changed with the **-h** flag: `-h=wss://localhost:8305`.

ℹ️ If the binary is running on a remote server don't forget to change the **Websocket Endpoint** setting of the Inox extension.

## Creating a Project

On you development machine create a `<myproject>` folder for the project.
Open the folder with Visual Studio Code, and execute the following command `Inox: Initialize new Project in Current Folder`.

Open the generated .code-workspace file and click on **Open Workspace**.


## Project Secrets

Project Secrets are **persisted** secrets, they can be created, updated & deleted from the **Inox Project** tab in VScode.

###  Retrieving project secrets

The global variable `project-secrets` is a global variable containing the secrets, it is only available from the main module.\
If you defined a secret named `MY_SECRET` you can retrieve it with the following code:
```
secret = project-secrets.MY_SECRET
```