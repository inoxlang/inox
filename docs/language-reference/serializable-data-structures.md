[Table of contents](./language.md)

---

# Serializable Data Structures

- [Lists](#lists)
- [Objects](#objects)
- [Tuples](#tuples)
- [Records](#records)
- [Treedata](#treedata)
- [Mappings](#mappings)
- [Dictionaries](#dictionaries)

**Serializable** data structures are intended to represent **business objects**
or part of them (e.g. a `user`).

Code performing heavy computations should use **transient types** such as
[Structs](./transient-types.md#structs) and [Arrays](./transient-types.md#arrays) instead.

## Lists

A list is a sequence of **serializable** elements. You can add elements to it
and change the value of an element at a given position.

```
list = []
list.append(1)

first_elem = list[0] # index expression
list[0] = 2

list = [1, 2, 3]
first_two_elems = list[0:2] # creates a new list containing 1 and 2

# lists can be spread in list literals.
other_list = [1, ...list]
```

Lists are **clonable** (see [Data Sharing](./concurrency.md#data-sharing)).

### Methods

- **append**
  ```
  list.append(1)
  list.append(1, 2, 3)
  ```

- **pop**
  ```
  list = [1, 2]
  removed_element = list.pop() # 2
  ```

- **dequeue**
  ```
  list = [1, 2]
  removed_element = list.dequeue() # 1
  ```
- **sorted**
  ```
  list = [3, 2, 1]

  # [1, 2, 3]
  sorted = l.sorted(#asc)
  ```
- **sort_by**
  ```
  list = [{count: 2}, {count: 1}]
  list.sort_by(.count, #asc)

  # [{count: 1}, {count: 2}]
  list
  ```

## Objects

An object is a serializable data structure containing properties, each property
has a name and a value.

```
object = {  
    a: 1
    "b": 2
    c: 0, d: 100ms
}

a = object.a
```

Implicit-key properties are properties that can be set without specifying a
name:

```
object = {
    1
    []
}

print(object)

output:
{
    "0": 1
    "1": []
}
```

Properties with an implicit key can be accessed thanks to an index expression,
the index should always be an integer:

```
object = {1}
one = object[0] # 1
1
```

Objects are **lock-protected** (see [Data Sharing](./concurrency.md#data-sharing)).

### Methods

Methods are defined in [extensions](./extensions.md), not in the objects.

### Computed Member Expressions

Computed member expressions are member expressions where the property name is
computed at runtime:

```
object = { name: "foo" }
property_name = "name"
name = object.(property_name)
```

⚠️ Accessing properties dynamically may cause security issues, this feature will
be made more secure in the near future.

### Optional Member Expressions

```
# if obj has a `name` property the name variable receives the property's value, nil otherwise.
name = obj.?name
```

### Object Spread

Object spreads require specifying the properties to spread.

```
object = {b: 2}
record = #{b: 2}

{a: 1, ...object.{b}}

# same result
{a: 1, ...record.{b}}
```

⚠️ Spreading a property that is already present is not allowed.

### Usage and Performance

Accessing the properties of an object is not fast and can even take several
milliseconds if the object is used by another thread.

Objects are not intended to be used when performing heavy computations. Use
[Structs](./transient-types.md#structs) instead.

## Records

Records are the immutable equivalent of objects, their properties can only have
immutable values.

```
record = #{
    a: 1
    b: #{ 
        c: /tmp/
    }
}

record = #{
    a: {  } # error ! an object is mutable, it's not a valid property value for a record.
}
```

## Tuples

Tuples are the immutable equivalent of lists.

```
tuple = #[1, #[2, 3]]

tuple = #[1, [2, 3]] # error ! a list is mutable, it's not a valid element for a tuple.

# tuples can be spread in tuple literals.
other_tuple = #[1, ...list]
```

## Treedata

A treedata value allows you to represent immutable data that has the shape of a
tree.

```
treedata "root" { 
    "first child" { 
        "grand child" 
    }   
    "second child"
    3
    4
}
```

<!-- In the shell execute the following command to see an example of treedata value ``fs.get_tree_data ./docs/`` -->

## Mappings

<!-- TODO: add explanation about static key entries, ... -->

A mapping maps keys and key patterns to values:

```
mapping = Mapping {
    0 => 1
    n %int => (2 * n)
    %/... => "path"
}

print mapping.compute(0)
print mapping.compute(1)
print mapping.compute(/e)

output:
1
2
path
```

## Dictionaries

Dictionaries are similar to objects in that they store key-value pairs, but
unlike objects, they allow keys of any data type as long as they are
representable (serializable).

```
dict = :{
    # path key
    ./a: 1

    # string key
    "./a": 2

    # integer key
    1: 3
}
```

[Back to top](#serializable-data-structures)
