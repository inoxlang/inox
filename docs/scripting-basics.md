[Install Inox](../README.md#installation) | [Language Basics](./language-basics.md) | [Shell Basics](./shell-basics.md) | [Built-in Functions](./builtin.md)

-----

# Scripting Basics

- [Hello World](#hello-world)
- [Example: Project Directory Generation](#example-project-directory-generation)
- [Retrieve, Filter & Extract Data](#retrieve-filter--extract-data)

## Hello World

```
manifest {}

print "hello world !"
```

An Inox program always starts with a **manifest**, the manifest lists:
- the permisssions required by the program
- the parameters of the program

You will learn more about the manifest throughout this tutorial.

Run the script using the following command:
```
inox run script.ix
```

### Shebang

Inox scripts support shebangs
- add `#!/usr/local/bin/inox run` at the top of the file
- `chmod u+x script.ix`
- ./script.ix


## Permissions

Let's learn about permissions by writing a script that creates a file:

```
manifest {
    permissions: {
        write: IWD_PREFIX
    }
}

create ./file.txt "hello world !"
```

The **permissions** section of the manifest lists the permissions required by our script.\
We need access to the filesystem so we added a write permission followed by the **IWD_PREFIX** constant (initial working directory)

> Note: the `write: IWD_PREFIX` permission allows writing to any file below the current directory: **./file.txt**, **./dir/file.txt ...**

## Example: Project Directory Generation

## Version 1

We will write a `gen-project.ix` script that takes 2 parameters:
- the location of the directory that will be created, it's a positional parameter (it is not prefixd with **--**).
- a `--clean-existing` switch to delete the directory if it already exists.


### Writing the Manifest

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

The script needs the `write` permission to create the directory structure and the `delete` permission to remove the directory if `--clean-existing` is present.\
Let's add the permissions section in the manifest

```
    permissions: {
        write: IWD_PREFIX # initial working directory
        delete: IWD_PREFIX
    }
```

### Writing the Logic

Now let's write the code for the script.
First we need to get the module's **arguments**:

```
dir = mod-args.dir
clean-existing = mod-args.clean-existing
```

If `clean-existing` is true we have to recursively remove the directory,
we can easily achieve this by using `fs.remove` or `delete`:

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

## Retrieve, Filter & Extract Data

Let's start by creating a **script.ix** file with the following content:

```
manifest {
    permissions: {
        read: %https://**
        create: {routines: {}}
    }
}
```

### Retrieve

The built-in [read](./builtin.md#read) function can read & parse the content of files, directories and HTTP resources.
You can learn more about resource manipulation [here](./shell-basics.md#resource-manipulation).

Throughout this tutorial, we will work with the mocked API served by https://jsonplaceholder.typicode.com/ :
```
assign posts err = read(https://jsonplaceholder.typicode.com/posts)
if err? { # if error not nil
    print("failed to retrieve data", err)
    return
}
```

We can ignore the error value by writing the following:
```
posts = read!(https://jsonplaceholder.typicode.com/posts)
```
ℹ️ Any error will be thrown instead, this type of call is a **must** call.

### Filter

The posts have a **.userId** field, let's just keep the posts of the user of id **1**:
```
assert (posts match %iterable)
user1_posts = filter(posts, %{userId: 1.0, ...})
```

**%{userId: 1.0, ...}** is a pattern that matches any object with the shape: **{userId: 1.0, ...}**.

Alternatively you can use a **lazy expression** to filter posts:
```
user1_posts = filter(posts, @($.userId == 1.0))
```

ℹ️ Surrounding an expression with @(...) creates a lazy expression that is evaluated for each post.

### Extract

The **map** function creates a list by applying an operation on each element of an iterable.
Let's call this function to extract the content of each post:
```
contents = map(user1_posts, .body)
```

Note: **.body** is a property name literal

We can extract several fields of the post by using a **key list**:
```
post_data = map(user1_posts, .{id, body, title})
```

Lazy expressions are another alternative here:
```
post_data = map(user1_posts, @({ 
    id: $.id
    title: $.title
    body: $.body
}))
```

⚠️ Lazy expressions are more flexible but are slower and less secure, try to only use them when necessary.

### Parallel Data Retrieval

Let's fetch the comments of each post by using the **/comments** endpoint.
We will use **coroutines** to retrieve the comments in parallel:

```
# we group the coroutines together
request_group = RoutineGroup()

for post in post_data {
    id = post.id
    assert (id match %float)

    go {group: request_group, globals: {id: toint(id)}} do read!(https://jsonplaceholder.typicode.com/comments?postId={id})
}

comments_of_user1_posts = request_group.wait_results!()
print(comments_of_user1_posts)
```

ℹ️ **go \[...]** is a spawn expression that creates a coroutine attached to **request_group**.

### Complete Script

```
manifest {
    permissions: {
        read: %https://**
        create: {routines: {}}
    }
}

POSTS_URL = https://jsonplaceholder.typicode.com/posts
posts = read!(POSTS_URL)
assert (posts match %iterable)

user1_posts = filter(posts, %{userId: 1.0, ...})
post_data = map(user1_posts, .{id, body, title})

# we group the coroutines together
request_group = RoutineGroup()

for post in post_data {
    id = post.id
    assert (id match %float)

    go {group: request_group, globals: {id: toint(id)}} do read!(https://jsonplaceholder.typicode.com/comments?postId={id})
}

comments_of_user1_posts = request_group.wait_results!()
print(comments_of_user1_posts)
```

### Simplification with Pipeline Statements

Let's simplify the following code a bit:
```
posts = read!(POSTS_URL)
assert (posts match %iterable)

user1_posts = filter(posts, %{userId: 1.0, ...})

post_data = map(user1_posts, .{id, body, title})
```

We can get rid of the **user1_posts** variable by using a [pipeline statement](./language-basics.md#pipe-statement):
```
posts = read!(POSTS_URL)
assert (posts match %iterable)

post_data = | filter $posts %{userId: 1.0, ...} | map $ .{id, body, title}
```
ℹ️ In pipeline statements **$** holds the result of the previous call.