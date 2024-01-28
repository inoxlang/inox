[Table of contents](./language.md)

---

# Testing

- [Basic](#basics)
- [Custom Filesystem](#custom-filesystem)
- [Program Testing](#program-testing)

Inox comes with a powerful testing engine that is deeply integrated with the
Inox runtime.

## Basics

A single test is defined by a **testcase** statement. Test suites are defined
using **testsuite** statements and can be nested.

```
manifest {}

testsuite "my test suite" {
    testcase "1 < 2" {
        assert (1 < 2)
    }
}

testsuite "my test suite" {
    testsuite "my sub test suite" {
        testcase "1 < 2" {
            assert (1 < 2)
        }
    }
}
```

Tests are allowed in any Inox file but it is recommended to write them in
`*.spec.ix` files. The modules whose filename matches `*.spec.ix` are granted
the following permissions by default in test mode:

- **read, write, delete all files**
- **read, write, delete all values in the ldb://main database**
- **read any http(s) resource**
- **create lightweight threads (always required for testing)**

The metadata and parameters of the test suites and test cases are specified in
an object:

```
manifest {}


testsuite ({
    name: "my test suite"
}) {

    testcase({ name: "1 < 2"}) {

    }

}
```

## Custom Filesystem

Test suites and test cases can be configured to use a short-lived filesystem:

```
manifest {}

snapshot = fs.new_snapshot{
    files: :{
        ./file1.txt: "content 1"
        ./dir/: :{
            ./file2.txt: "content 2"
        }
    }
}

testsuite ({
    # a filesystem will be created from the snapshot for each test suite and test case.
    fs: snapshot
}) {

    assert fs.exists(/file1.txt)
    fs.rm(/file1.txt)

    testcase {
        # no error
        assert fs.exists(/file1.txt)
        fs.rm(/file1.txt)
    }

    testcase {
        # no error
        assert fs.exists(/file1.txt)
    }
}
```

Test suites can pass a copy of their filesystem to subtests:

```
testsuite ({
    fs: snapshot
    pass-live-fs-copy-to-subtests: true
}) {
    fs.rm(/file1.txt)

    testcase {
        # error
        assert fs.exists(/file1.txt)
    }

    testcase {
        # modifications done by test cases have no effect for subsequent tests
        # because they are given a copy of the test suite's filesystem.
        fs.rm(/file2.txt)
    }

    testcase {
        # no error
        assert fs.exists(/file2.txt)

        # error
        assert fs.exists(/file1.txt)
    }

}
```

## Program Testing

Inox's testing engine is able to launch an Inox program/application. Test suites
and test cases accept a **program** parameter that is inherited by subtests. The
program is launched for each test case in a short-lived filesystem.

```
manifest {
    permissions: {
        provide: https://localhost:8080
    }
}

testsuite({
    program: /web-app.ix
}) {
    testcase {
        assert http.exists(https://localhost:8080/)
    }

    testcase {
        assert http.exists(https://localhost:8080/about)
    }
}
```

The short-lived filesystem is created from the current project's
[base image](#project-images).

**Database initialization**:

The main database of the program can be initialized by specifying a schema and
some initial data:

```
manifest {
    permissions: {
        provide: https://localhost:8080
    }
}

pattern user = {
    name: str
}

testsuite({
    program: /web-app.ix
    main-db-schema: %{
        users: Set(user, #url)
    }
    # initial data
    main-db-migrations: {
        inclusions: :{
            %/users: []
        }
    }
}) {
    testcase "user creation" {
        http.post!(https://localhost:8080/users, {
            name: "new user"
        })

        db = __test.program.dbs.main
        users = get_at_most(10, db.users)

        assert (len(users) == 1)
        assert (users[0].name == "new user")
    }
}
```

[Back to top](#testing)
