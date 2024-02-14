# Limits

Limits limit intensive operations, there are three kinds of limits:
**[byte rate](#byte-rate-limits)**, **[frequency](#frequency-limits)** &
**[total](#total-limits)**. Limits are defined in module manifests.

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

## Sharing

At runtime a counter will be created for each limit, the behaviour of the
counter is specific to the limit's kind. Limits defined by a module will be
shared with all of its child modules/threads. In other words when the module
defining the limit or one if its children performs an operation a shared counter
is decremented.

**Example 1 - CPU Time**

```
# ./lib.ix
manifest {}

do_intensive_operation1()
...
return ...


# ./main.ix
manifest {
    limits: {
        "execution/cpu-time": 1s
    }
}

# all CPU time spent by the lib is added to the counter of ./main.ix
import lib ./lib.ix {} 

# all CPU time spent by the child threads are added to the counter of ./main.ix
lthread = go do {
    do_intensive_operation2()
}

...
```

[Issues with the CPU time limit.](https://github.com/inoxlang/inox/issues/19)

**Example 2 - Simultaneous Thread Count**

```
# ./main.ix
manifest {
    limits: {
        "threads/simul-instances": 2
    }
}

# lthread creation, the counter is decreased by one
lthread = go do {
    # lthread creation inside the child lthread, the counter is decreased by one
    go do {
        sleep 1s
    }
    sleep 1s
}

# at this point 2 lthreads are running, attempting to create a new one would cause an error.
...
```

## Byte Rate Limits

This kind of limit represents a number of bytes per second.\
Examples:

- `fs/read`
- `fs/write`

## Frequency Limits

This kind of limit represents a number of operations per second.\
Examples:

- `fs/create-file`
- `http/request`
- `object-storage/request`

## Total Limits

This kind of limit represents a total number of operations or resources.
Attempting to make an operation while the counter associated with the limit is
at zero will cause a panic.\
Examples:

- `fs/total-new-files` - the counter can only go down.
- `ws/simul-connections` - simultaneous number of WebSocket connections, the
  counter can go up & down since connections can be closed.
- `execution/cpu-time` - the counter decrements on its own, it pauses when an IO
  operation is being performed.
- `execution/total-time` - the counter decrements on its own.
