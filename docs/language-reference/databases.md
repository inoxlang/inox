[Table of contents](./language.md)

---

# Databases

- [Schema](#database-schema)
- [Serialization](#serialization)
- [Access From Other Modules](#access-from-other-modules)


Inox comes with an embedded database engine, you can define databases in the
manifest:

```
manifest {
    permissions: {}
    databases: {
        main: {
            #ldb stands for Local Database
            resource: ldb://main 
            resolution-data: nil
        }
    }
}
```

## Database Schema

The schema of an Inox Database is an [object pattern](./patterns#object-patterns), it can
be set by calling the **update_schema** method on the database:

```
pattern user = {
  name: str
}

dbs.main.update_schema(%{
    users: Set(user, #url)
})
```

⚠️ Calling **.update_schema** requires the following property in the database
description: `expected-schema-update: true`.

The current schema of the database determinates what values are accessible from
`dbs.main`.\
If the current schema has a `users` property the users will be accessible by
typing `dbs.main.users`.

You can make the typesystem pretend the schema was updated by adding the
**assert-schema** property to the database description.\
For example if the database has an empty schema, you can use the following
manifest to add the `users` property to the database value:

```
preinit {
    pattern schema = %{
        users: Set(user, #url)
    }
}

manifest {
    permissions: {}
    databases: {
        main: {
            resource: ldb://main 
            resolution-data: nil
            assert-schema: %schema
        }
    }
}

users = dbs.main.users
```

⚠️ At runtime the current schema of the database is matched against
`assert-schema`.\
So before executing the program you will need to add a call to
`dbs.main.update_schema` with the new schema.

## Migrations

Updating the schema often requires data updates. When this is the case
`.updated_schema` needs a second argument that describes the updates.

```
pattern new_user = {
    ...user
    new-property: int
}

dbs.main.update_schema(%{
    users: Set(new_user, #url)
}, {
   inclusions: :{
        %/users/*/new-property: 0

        # a handler can be passed instead of an initial value
        # %/users/*/new-property: fn(prev_value) => 0
    }
})
```

| Type                | Reason                                           | Value, Handler Signature                     |
| ------------------- | ------------------------------------------------ | -------------------------------------------- |
| **deletions**       | deletion of a property or element                | **nil** OR **fn(deleted_value)**             |
| **replacements**    | complete replacement of a property or element    | **value** OR **fn(prev_user user) new_user** |
| **inclusions**      | new property or element                          | **value** OR **fn(prev_value) new_user**     |
| **initializations** | initialization of a previously optional property | **value** OR **fn(prev_value) new_user**     |

ℹ️ In application logic, properties can be added to database objects even if they
are not defined in the schema.\
During a migration previous values are passed to the handlers.

## Serialization

Most Inox types (objects, lists, Sets) are serializable so no translation layer
is needed to add/retrieve objects to/from the database.

```
new_user = {name: "John"}
dbs.main.users.add(new_user)

# true
dbs.main.users.has(new_user)
```

Since most Inox types are serializable they cannot contain transient values.

```
object = {
  # error: non-serializable values are not allowed as initial values for properties of serializables
  lthread: go do {
    return 1
  }
}

# same error
list = [  
  go do { return 1 }
]
```

[Arrays](./transient-types.md#arrays) and [Structs](./transient-types.md#structs) can contain any value.

### URLs

Objects and many container types are assigned a URL when they are added to a
container inside the database.

```
user = {name: "John"}
dbs.main.users.add(user)

# user now has a URL of the following format:
ldb://main/users/<id>
```

### Cycles and References (WIP)

The serialization of values containing cycles is forbidden. URLs should be used
to reference other values inside the database.

```
pattern user = {
    name: str
    friends: Set(%ldb://main/users/%ulid, #repr) 
}

user = ... # get a user from the database

new_friend = {name: "Enzo"}
user.friends.add(new_friend)

print("friends:")

for friend in user.friends {
    # load the friend's name by using the URL in a double-colon expression:
    name = friend::name
    print(name)

    # the friend's object can be loaded using the built-in get function:
    loaded_friend = get!(friend)
}
```

## Access From Other Modules

If the `/main.ix` module defines a `ldb://main` database, imported modules can
access the database with the following manifest:

```
manifest { 
    databases: /main.ix
    permissions: {
        read: {
            ldb://main
        }
        # you can also add the write permission if necessary
    }
}

for user in dbs.main.users {
    print(user)
}
```

ℹ️ The module defining the databases is automatically granted access to the
database.

⚠️ Permissions still need to be granted in the import statement.

[Back to top](#)
