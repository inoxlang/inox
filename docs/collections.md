# Collections

**⚠️ The current implementations are work in progress. Performance and locking will be
improved in the future.**

- [Set](#set)
- [Map](#map)
- [MessageThread](#message-thread)
- [Queue](#queue)
- [Tree](#tree)
- [Graph](#graph)
- [Ranking](#ranking)


Inox collections can be either **persistable** or **transient**.

## Set

- **Persistable**
- **Sharable**
- **Unique elements**
- **Serializable elements only**

A **Set** is an unordered collection with no duplicate elements. Several
`uniqueness types` are supported:

- [Representation](#representation-uniqueness) (default)
- [Property value](#property-value-uniqueness)
- [URL](#url-uniqueness)

### Methods

The `add` method adds an element to the set. Adding the **exact same element**
twice or more is allowed.

```
set = Set([])
set.add(1)
```

The `remove` method removes an element from the set. It is safe to pass a value
that is not part of the set (nothing will happen).

```
set.remove(1)
```

The `has` method tells whether the argument is an element of the set.

```
boolean = set.has(1)
```

### Representation Uniqueness

Elements with the same JSON representation are considered the same. As the
representation of certain data structures can change (e.g. objets), only
immutable Inox values are allowed.

```
set = Set([], {element: %record, unique: #repr})

# The two following records are considered the same value by the Set.
record1 = #{a: 1}
record2 = #{a: 1}

set.add(record1)

set.has(record2)
```

All simple values (e.g. **integers**, **floats**, **strings**) are immutable,
therefore they can be stored in the set.

```
integers = Set([], {element: %int, unique: #repr})

integers.add(1)
```

#### Default configuration

If no configuration is provided the Set defaults to a **representation
uniqueness** and accepts all immutable serializale values.

```
set = Set([])

record = #{a: 1}

set.add(record)
set.add(1)
```

#### Transaction and Locking

In the representation uniqueness configuration, read-write transactions have to
acquire the set in order to interact with it. Other read-write transactions have
to **wait** for the previous transaction to finish before attempting to acquire
the set. Readonly transactions can read the Set while it is not acquired by a
read-write transaction.

### Property Value Uniqueness

In this configuration each element has a unique value for a given property.
Adding an element with the same property value as another element is not
allowed.

```
set = Set([], {element: object, unique: .name})


userA = {name: "user A"}
userB = {name: "user B"}

set.add(userA)

other_user = {name: "User A"}

set.has(other_user) # false
set.add(other_user) # runtime error
```

#### Transaction and Locking

In the property-value uniqueness configuration, read-write transactions have to
acquire the set in order to interact with it. Other read-write transactions have
to **wait** for the previous transaction to finish before attempting to acquire
the set. Readonly transactions can read the Set while it is not acquired by a
read-write transaction.

### URL Uniqueness

This configuration is only supported by sets persisted in databases, and
elements must be able to have a URL (e.g. objects). Added elements are given a
URL based on the set's database URL.

```
pattern user = {
    name: str
}

pattern db-schema = {
    users: Set(user, #url)
}

user1 = {name: "user A"}
set.add(user1)

user2 = {name: "user A"}

set.has(user2) # false
set.add(user2) # no error
```

⚠️ **Adding to the set an element from another URL-based set is not allowed.**

#### Transaction and Locking

(temporary)

In the URL uniqueness configuration, read-write transactions have to acquire the
set in order to interact with it. Other read-write transactions have to **wait**
for the previous transaction to finish before attempting to acquire the set.
Readonly transactions can read the Set while it is not acquired by a read-write
transaction.

(future)

Read-write transactions will be able to add (create) elements without having to
acquire the Set. The set uses ULIDs as identifiers for elements, so it's
virtually impossible for different transactions running at the same time to add
the same element.

## Map

- **Persistable**
- **Sharable**
- **Unique keys**
- **Serializable elements only**

A **Map** is an unordered collection with key-value pairs, keys are unique and
immutable.

```
map = Map(["A", 1, "B", 2], {key: %str, value: %int})

map.set("C", 3)
```

### Methods

The `insert` method creates a new entry in the map. If the entry (same key) already exists
the function panics.

```
map = Map([])
map.insert('a', 1)

# panic
set.insert('a', 1)
```

The `set` method creates or updates an entry. 

```
map = Map([])
map.set('a', 1)
map.set('a', 1) # allowed
map.set('a', 2) # allowed
```

The `remove` method removes an entry by its key. 

```
map.remove('a')
```

The `get` method returns the value of an entry.

```
map.set('a', 1)
one = map.get('a')
```

## Message Thread

- **Persistable**
- **Sharable**
- **Object elements only**

A **Message Thread** is a thread of 'messages'. A message can have any (object) type: it can represent a regular message, an email or a notification.

**Message threads can only be created in databases, there is no factory function or constructor that you can call.**

### Methods

The `add` method adds a message to the thread and gives it a database URL.

```
pattern message = {
    text: string
}

pattern db-schema = {
    thread: MessageThread(message)
}

...

message = {
    text: "hello"
}

dbs.main.thread.add(message)
```

## Queue

WIP

## Tree

WIP

## Graph

WIP

## Ranking

WIP

