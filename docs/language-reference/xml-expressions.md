[Table of contents](./README.md)

---

# XML Expressions

An XML expression produces a value by passing an XML-like structure to a
namespace that interprets it:

```
# The XML structure is passed to the html namespace.
html<div></div> 

# The namespace can be omitted if the expression is parenthesized.
# The implicit namespace is always html.
string = "world"
element = (<div> Hello {string} ! </div>)

# Self closing tag
(<img src="..."/>)
```

In the `<script>` and `<style>` tags, anything inside single brackets is not
treated as an interpolation:

```
html<html>
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

[Back to top](#xml-expressions)
