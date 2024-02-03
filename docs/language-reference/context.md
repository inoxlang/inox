# Context

Each module instance has an **Inox context**: a runtime component that stores specific kinds of data and is responsible
for checking [permissions](./permissions.md) and [limit](./modules.md#limits) resource usage (limits).

- [Context Data](#context-data)
- [Host Definitions](#host-definitions)
- [Cancellation](#cancellation)
    - [Graceful](#graceful)
    - [Ungraceful](#ungraceful)
    - [Manual Cancellation](#manual-cancellation)


## Context Data

Context data entries are indexed using **paths** , each entry can be defined only once. The value should fall in one of the following categories: **sharable** (e.g. objects, sets), **immutable** (e.g. integers, records)
or **clonable** (e.g. lists). 

Child modules have access to the context data of their parent and can individually override entries.

```
# Add the context data entry /x with 1 as value.
add_ctx_data(/x, 1)

# Retrieve the value of the entry /x.
value = ctx_data(/x)

# Retrieving the value of an undefined entry returns nil.
print("undefined entry /y:", ctx_data(/y))

# Retrieve the value of the entry /x and check that the value is an integer.
value = ctx_data(/x,  %int)

# Child modules have access to the context data of their parent and can override the entries.
lthread = go do {
    print("/x from parent:", ctx_data(/x))

    add_ctx_data(/x, 2)

    print("overriden /x:", ctx_data(/x))
}

lthread.wait_result!()
```

[Documentation for add_ctx_data](../builtins.md#add_ctx_data)\
[Documentation for ctx_data](../builtins.md#ctx_data)

## Host Definitions

The resolution data of hosts is stored inside the context.

```
manifest {
   	host-resolution: :{
		ldb://main : /mydb
	}
}
```

Child contexts inherit all hosts definitions from their parent.

## Cancellation

The cancellation of a context causes the associated module instance to **stop its execution**, and descendant modules are recursively cancelled as well. There are two kinds of cancellations: **graceful** and **ungraceful**.

### Graceful

During a **graceful cancellation**, teardown handlers are executed. Graceful teardown handlers can be registered by any internal source that needs to perform some cleanup (e.g. a database needing to be closed). After the graceful teardown **'done'** microtasks are called. As the name implies those **microtasks** are very short and only perform limited cleanup operations.

### Ungraceful

**During an ungraceful cancellation only 'done' microtasks are called.**

### Manual cancellation

```
cancel_exec()
```