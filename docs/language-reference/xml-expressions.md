[Table of contents](./README.md)

---

# XML Expressions

An XML expression produces a value by passing a XML-like structure to a
namespace that interprets it:

```
string = "world"
element = (<div> Hello {string} ! </div>)

# Self closing tag
(<img src="..."/>)


# Parentheses can be omitted by prefixing the expression with `html`
html<div></div>
```

In the `<script>` and `<style>` tags, anything inside single brackets is not
treated as an interpolation:

```
html<html>
    <style>
        html, body {
            margin: 0;
        }
    </style>
    <script>
        const object = {a: 1}
    </script>
</html>
```

[Back to top](#xml-expressions)
