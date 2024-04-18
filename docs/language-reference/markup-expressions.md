[Table of contents](./README.md)

---

# Markup Expressions

A markup expression produces a value by passing markup to an Inox namespace that interprets it.

```
# The markup is passed to the html namespace.
html<div></div> 

The namespace is optional and defaults to html if not explicitly specified.
<div></div> 

# Interpolation
string = "world"
element = <div> Hello {string} ! </div>

# Self closing tag
<img src="..."/>
```

## Special Elements

In `<script>` and `<style>` elements, anything inside single brackets is treated as text:

```
<html>
    <style>
        html, body { # not an interpolation
            margin: 0;
        }
    </style>
    <script>
        const object = {a: 1}
    </script>
</html>
```

> Any Inox namespace having a member `from_markup_elem` (function) can be used to interpret a markup expression.

## Hyperscript Attribute Shorthand

In opening tags Hyperscript code can be written between curly braces instead of with a regular attribute (`_="..."`).

```html
<div {on click toggle .red on me}></div>
      |
interpreted as
      |
      v
<div _="on click toggle .red on me"></div>
```

Hyperscript attribute shorthands can span several lines:

```html
<div {
    on click toggle .red on me
}>
</div>
```

**If you want to write other attributes after the shorthand a dot '.' is required after the closing brace.**

```html
<div 
    {on click toggle .red on me}.
    class=""
>
    ...
</div>
```

[Back to top](#markup-expressions)
