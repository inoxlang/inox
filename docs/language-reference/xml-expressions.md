[Table of contents](./README.md)

---

# XML Expressions

An XML expression produces a value by passing a XML-like structure to a
namespace that interprets it:

```
string = "world"
element = html<div> Hello {string} ! </div>

# self closing tag
html<img src="..."/>
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
