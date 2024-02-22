[Back to README](../README.md)

---

# Frontend Development

- 📄 [Pages](#pages)
- ⚙️ [Server-Side Components](#server-side-components)
- 🌐 [Client-Side Components](#client-side-components---inoxjs)
- 📝 [Forms](#forms)
- ✨ [Planned HTML & HTMX integrations](#planned-html-and-htmx-integrations)

The frontend of an Inox application is built using the following features and
libraries:

- The `filesystem routing` feature of the HTTP server executes modules returning
  the HTML of pages and server side components.
- [HTMX](https://htmx.org/) allows any HTML element to issue an HTTP request,
  enabling dynamic content updates in web applications without the complexity of
  heavy JavaScript frameworks.
- [Inox.js](#client-side-components---inoxjs) is a **tiny** experimental
  library allowing to develop small client-side components when HTMX is not a
  good fit. You can use another library if you prefer to.

```
client/      --- client side components
    counter.ix

components/  --- server side components
    login-form.ix

routes/      --- pages and API(s)
    index.ix
    last-news.ix
    users/
        GET.ix
        POST.ix

static/
    base.css
    htmx.min.js
    inox.js
```

## Pages

Inox's HTTP server supports [Filesystem routing](./http-server-reference.md#filesystem-routing).

| Path          | HTTP method | Handler paths (recommended)                           |
| ------------- | ----------- | ----------------------------------------------------- |
| `/`           | `GET`       | `/routes/index.ix`                                    |
| `/about`      | `GET`       | `/routes/about.ix , /routes/about/index.ix`           |
| `/about/team` | `GET`       | `/routes/about/team.ix`                               |
| `/users`      | `POST`      | `/routes/users/POST.ix , /routes/users/POST-users.ix` |
| `/users/0`    | `POST`      | `/routes/users/:user-id/POST.ix`                      |
| `/users/0`    | `DELETE`    | `/routes/users/:user-id/DELETE.ix`                    |


```html
# /routes/index.ix
manifest {}

return html<html>
<head>
    <meta charset="utf-8"/>
    <title></title>
    <meta name="viewport" content="width=device-width, initial-scale=1"/>
    <link rel="stylesheet" href="/base.css"/>
    <script src="/htmx.min.js"></script>
    <script src="/inox.js"></script>
</head>
<body>
    <header> index.ix </header>

    <section>
        <header> Last news </header>

        <!-- on load HTMX fetches the content of /last-news and inserts it in the page -->
        <div hx-get="/last-news" hx-trigger="load"></div>
    </div>
</body>
</html>
```

---

## Server Side Components

```html
# /routes/last-news.ix
manifest {}

return html<ul>
    <li>News 1</li>
    <li>News 2</li>

    <!-- Local styling enabled by the CSS Scope Inline library (included in inox.js) -->
    <style>
        me {
            display: flex;
            flex-direction: column;
            ...
        }
    </style>
</ul>
```

**The previous code can also be turned into a function
`fn(){ return html<ul>...</ul> }` and used in several places.**

---

## Client-Side Components - Inox.js

Each Inox project comes with a `/static/` folder that contains, among other
things, a small experimental library that allows creating client-side components
with locality of behavior. This library updates a component's view when the state
changes. It is packaged with the following micro libraries (all MIT licensed):

- Preact Signals: https://github.com/preactjs/signals/tree/main/packages/core (<
  900 lines)
- CSS Scope Inline: https://github.com/gnat/css-scope-inline (< 20 lines)
- Surreal: https://github.com/gnat/surreal (< 400 lines)

**It is recommended to use client-side components only for functionalities that
can't be easily implemented with Server-Side Rendering (SSR) and HTMX. The
following example is only provided as a demonstration.**

```html
# /client/counter.ix
includable-file

fn Counter(){
    return html<div class="counter">
        <div class="status">
            <span>Count:</span>
            <!-- safe text-only interpolations with default values -->
            <span> $(count:'0') double: $(double:'0') </span>
        </div>

        <div class="actions">
            <button class="increment">Increment</button>
            <button class="decrement">Decrement</button>
        </div>

        <script> 
        {
            const count = signal(0);
            const double = computed(() => count.value * 2);

            // initComponent is provided by inox.js. This function initializes the component in order 
            // to update the view when signals change.
            initComponent({ signals: {count, double} })

            // The 'me' function is provided by the Surreal library and returns the div element with 
            // the .counter class.
            me(".increment").on('click', () => {
               count.value++
            })    

            me(".decrement").on('click', () => {
                count.value = Math.max(0, count.value-1)
            })    
        }
        </script>

        <!-- Local styling of the counter -->
        <style>
            me {
                width: 250px;
                padding: 7px;
                border-radius: 3px;
                border: 1px solid grey;
                display: flex;
                flex-direction: column;
                border-radius: 5px;
                align-items: center;
            }

            me :matches(.status, .actions) {
                display: flex;
                flex-direction: row;
                gap: 5px;
            }

            me button {
                font-size: 15px;
                border-radius: 5px;
                background-color: lightgray;
                padding: 2px 15px;
                cursor: pointer;
            }

            me button:hover {
                filter: brightness(1.1);
            }
        </style>
    </div>
}
```

### Planned Features

> inox.js will stay minimal: specific features will be provided by extensions.

**Conditional rendering**

```html
<div x-if="count == 100">Max count reached</div>

<div x-switch>
    <div x-case="count > 50">Count is high</div>
    <div x-case="count > 90">Count is dangerously high</div>
    <div x-case="count == 100">Max count reached</div>
</div>
```

---

## Forms

For now Inox's HTTP server only accepts JSON as the content type of
`POST | PATCH | PUT` requests. Therefore **forms** making requests to it are
required to have specific attributes that enable JSON encoding.

```html
<form hx-post-json="/users">
    ...
</form>
```

**OR (equivalent)**

```html
<form hx-post="/users" hx-ext="json-form">
    ...
</form>
```

### Encoding

- The values of `number` and `range` inputs are converted to numbers.
- The values of `checkbox` inputs with a `yes` value are converted to booleans.
- The values of checked `checkbox` inputs are gathered in an array, even if
  there is a single element.
- The values of inputs whose name contains an array index (e.g.
  `elements[0], elements[1]`) are gathered in an array.
- The values of inputs whose name contains a property name (e.g.
  `user.name, user.age`) are put into an object.

```html
<input name="username" type="text">     
--> {"username": (string)}

<input name="count" type="number">      
--> {"count": (number)}

<input name="enable" type="checkbox" value="yes">
--> {"enable": (boolean)}

<input name="choices" type="checkbox" value="A">
<input name="choices" type="checkbox" value="B">
--> {"choices": (array)}

<input name="elements[0]" type="text">
<input name="elements[1]" type="text">
--> {"elements": (array)}

<input name="user.name" type="text">
<input name="user.age" type="number">
--> {
    "user": {
        "name": (string),
        "age": (number)
    }
}

<input name="elements[0].name" type="text">
<input name="elements[1].name" type="text">
--> {
    "elements": [
        {"name": (string)},
        {"name": (string)}
        ...
    ]
}
```

### Directly Specifying A Payload

You can use the `jsonform-payload` attribute to specify the JSON payload directly.

```html
fn TodoItem(){
    payload = asjson({
        updates: [{ key: item.key, done: !item.done}]
    })

    return html<div>
        <form hx-patch-json="/todos" jsonform-payload=payload>
            <button>
                {(if item.done "✅" else "⬜")}
            </button>
        </form>

        ...
    </div>
}
```

## Planned HTML and HTMX Integrations

**Implementation has begun.**

### Checks

- Validation of `<input>` elements in forms against the current API.
- Validation of URLs in attributes such as `hx-get` against the current API.

_and more._

### LSP

- `<form>` and `<input>` completions based on the current API.
- URL completion for attributes such as `hx-get`.

_and more._

---

[Back to README](../README.md)
