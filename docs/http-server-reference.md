# HTTP Server Reference

**WORK IN PROGRESS**

- [Creation]
  - [Handler function](#handler-function)
- [Certificate]
- [Filesystem routing](#filesystem-routing)
- [Mapping handler](#mapping-handler)
- [Request handling](#request-handling)

## Creation

The builtin `http.Server` function creates a listening HTTP**S** server.\
The first parameter is the **listening address**: it should be a HTTPS host such
as `https://localhost:8080` or `https://0.0.0.0:8080`.

```
server = http.Server!(https://localhost:8080)
```

The second parameter is a handler ([function](#handler-function) or
[mapping](#mapping-based-routing)), or a
[configuration object](#configuration-object).

## Certificate

Inox's HTTP server

## Configuration Object

## Filesystem Routing

```
server = http.Server!(https://localhost:8080, {
    routing: {
        static: /static/
        dynamic: /routes/
    }
})
```

When the server receives a request it determines what is the handler module
(file) for the request, and invokes the handler. The routing rules are the
following:

| Request's path | HTTP method | Possible handler paths                                        |
| -------------- | ----------- | ------------------------------------------------------------- |
| `/`              | `GET`         | `/GET-index.ix , /index.ix`                                   |
| `/about`         | `GET`         | `/GET-about.ix , /about.ix , /about/GET.ix , /about/index.ix` |
| `/users`         | `POST`        | `/POST-users.ix , /users/POST.ix , /users.ix`                  |
| `/users/0`       | `POST`        | `/users/:user-id/GET.ix , /users/:user-id/index.ix`           |

A **single handler module** should be defined for each endpoint/METHOD pair. The `http.Server` function panics
if there are two or more handlers for the same endpoint/METHOD pair.

### Context Data

The filesystem router adds a context data entry for each **path parameter**. 
The **key** of the entry is `/path-params/<parameter name>`.

Let's say that we have the following filesystem structure:


```
routes/
    users/
        :user-id/
            GET.ix
```


If a request of path `/users/123` is received the `GET.ix` handler module will be invoked.
The call to `ctx_data(/path-params/user-id)` will return the string `123`.


## Handler Function

**WORK IN PROGRESS**

```
fn handle(response-writer http.resp-writer, request http.req){
    match request.path {
        / {
            response-writer.write_html("<!DOCTYPE html><html>..</html>)
        }
    }
}

http.Server(ADDR, handle)
```

## Mapping Handler

**WORK IN PROGRESS**

A **Mapping** handler can be used to handle simple requests or route requests.

```
fn handle(response-writer http.resp-writer, request http.req){
  ...
}

server = http.Server!(ADDR, Mapping {
    /hello => "hello"
    %/... => handle
})
```

## Request Handling

### 1. Pre-Validation

The pre-validation step perform several simple basic checks on the request. If
the pre-validation fails a response with a `404` status (Bad Request) is sent.

- The method should be a valid HTTP method (GET, POST, ...).
- A `Content-Type` header should be present for HTTP methods with a body.
- The path should not contain `..` segments, that includes `/../` and `\..\`

### 2. Rate Limiting

The server's security engine determines if the request should be rate limited.
Rate limited requests receive a response with a `429` status code.

### 3. State and Transaction Creation

A state is created for the handler. A **transaction** is also created if the
request does not accept a response of type `text/event-stream`. The transaction
times out after `20s` by default.

### 4. Routing

This step depends on the routing method:

- [Filesystem routing](#filesystem-routing)
- [Mapping handler](#mapping-handler)
- Manual routing if the handler is a [function](#handler-function)

### 5. Handler Invocation

The handler is invoked, it can be a **function**, a **Mapping** or a **module**.

- Handler functions are executed using the state created at
  [step 3](#3-routing).
- Mapping computations and nested handlers are executed with
  the state created at [step 3](#3-routing).
- The handler module selected by the **filesystem router** is executed using a
  state created by **preparing** the module. It is a child of the state at
  [step 3](#3-routing).
