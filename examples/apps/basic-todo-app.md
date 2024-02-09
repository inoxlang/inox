# Basic ToDo App

Note: The application uses [HTMX](https://htmx.org/), and a [custom HTMX extension](../../docs/frontend-development.md#forms) for converting form parameters to JSON. The current state of the code is not yet representative of all the features that Inox will provide. Also please note that 
this is a limited application example.

![basic-todo-app-demo](https://github.com/inoxlang/inox/assets/113632189/aeddd860-d73e-4285-87ed-5efbdb04e726)

---

**Files**

- [main.ix](#mainix)
- [schema.ix](#schemaix)
- [app.spec.ix](#appspecix)
- **routes/**
  - [index.ix](#routesindexix)
  - **users/**
    - [POST.ix](#routessessionspostix)
  - **sessions/**
    - [POST.ix](#routessessionspostix)
  - **todos/**
    - [GET.ix](#routestodosgetix)
    - [POST.ix](#routestodospostix)
    - [PATCH.ix](#routestodospatchix)
- **components/**
  - [common.ix](#componentscommonix)
  - [login.ix](#componentsloginix)
  - [todo.ix](#componentstodoix)
- **static**
  - **base.css**
  - **htmx.min.js**
  - **inox.js** (< 12kB gzipped, not minified yet),
    - [Surreal](https://github.com/gnat/surreal) 
    - [CSS Scope Inline](https://github.com/gnat/css-scope-inline)
    - [Preact Signals](https://github.com/preactjs/signals/tree/main/packages/core)
    - [Small client-side component library](../../docs/frontend-development.md#client-side-components---inoxjs)

## /main.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/b64cdf09-8cf6-4ad5-b521-fb88d39c4b9c)
</details>


## /schema.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/5cf17dc6-e4cd-4df9-9701-79c77f1efcfd)
</details>

## /app.spec.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/e2457153-8470-4287-9ae5-292fa56a5ff9)
</details>



## /routes/index.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/3f6c9877-6bf7-4a69-9ad2-1d9f60e04782)
</details>

## /routes/users/POST.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/a6ab983c-7868-47d5-8084-3cbcd999a04b)
</details>


## /routes/sessions/POST.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/a79f226f-1120-474a-8a3d-68bd3a7f59db)
</details>

## /routes/todos/GET.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/57681781-f73e-4b03-ba12-ad2aec0c1390)
</details>


## /routes/todos/POST.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/52e57868-841a-4dca-93ad-f6f386af4e80)
</details>


## /routes/todos/PATCH.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/e72db633-60e9-4faa-b0b0-34d397a1a7b6)
</details>

## /components/common.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/4dfb191d-a440-46dd-8ef5-4472d04b281f)
</details>

## /components/login.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/cb3d0e88-8715-4163-b2f7-f15ef4cbbc29)
</details>

## /components/todo.ix

<details>

![image](https://github.com/inoxlang/inox/assets/113632189/1e777959-a863-497c-9e1b-82d42728fa32)
</details>


[Go to top](#basic-todo-app)

