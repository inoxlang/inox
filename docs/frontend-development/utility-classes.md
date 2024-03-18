# Utility Classes

Inox comes with a subset of [Tailwind](https://tailwindcss.com/) and a small utility class system based on CSS variables.

- The provided Tailwind's subset does not require reading the official documentation and it can be used without any configuration. An optional configuration system will likely be supported in the future.
- Theming is implemented using CSS variables, not Tailwind.
- Used utility classes are automatically added to the file `/static/css/utility-classes.gen.css`.

**Documentation sections**:

- [Tailwind](#tailwind)
    - [Breakpoint modifiers](#breakpoint-modifiers)

- [Variable-Based Utilities](#variable-based-utilities)
    - [Variable definitions and theming](#variable-definitions-and-theming)
    - [Property inference](#property-inference-rules)

<details>

_<summary>✨ You can hover a utility class to see its associated rule.</summary>_

![ezgif-6-c4870884fa](https://github.com/inoxlang/inox/assets/113632189/228cc727-de00-4521-b058-721273355647)

</details>

## Tailwind

Inox supports the following subset of Tailwind: https://github.com/inoxlang/tailwind-subset/tree/main/src and a few modifiers.

### Breakpoint Modifiers

| Breakpoint prefix | Minimum width | Minimum width	CSS                     |
| ----------------- | ------------- | ------------------------------------ |
| `sm`              |  `640px`        | `@media (min-width: 640px) { ... }`  |
| `md`              |  `768px`        | `@media (min-width: 768px) { ... }`  |
| `lg`              |  `1024px`       | `@media (min-width: 1024px) { ... }` |
| `xl`              |  `1280px`       | `@media (min-width: 1280px) { ... }` |
| `2xl`             |  `1536px`       | `@media (min-width: 1536px) { ... }` |


❌ Don’t use `sm:` to target mobile devices.
```html
<!-- This will only center text on screens 640px and wider, not on small screens -->
<div class="sm:text-center"></div>
```

✅ Use unprefixed utilities to target mobile, and override them at larger breakpoints.

```html
<!-- This will center text on mobile, and left align it on screens 640px and wider -->
<div class="text-center sm:text-left"></div>
```

**More breakpoint modifiers will be supported.**

## Variable-Based Utilities

Variable-based utilities are CSS classes whose name starts with `--`, for example: `--primary-bg`.
Inox infers a rule on a CSS property based on the name of the variable.

Let's assume an Inox codebase contains the following piece of markup:

```html
<div class="--primary-bg"></div>
```

Inox detects that `--primary-bg` is a variable-based class and sees that it contains the substring `bg` for **background**.
So it adds a rule to `/static/css/utility-classes.gen.css`:

```css
.--primary-bg {
    background: var(--primary-bg);
}
```


### Variable Definitions And Theming

Inox projects contain a file `/static/css/variables.css` by default.
This file defines [CSS variables](https://developer.mozilla.org/fr/docs/Web/CSS/Using_CSS_custom_properties) (custom properties).

```css
:root {
    --link-fg: #0076c6;
    --border-radius: 4px; 

    --font-size-small: 12px; 
    --font-size-medium: 16px;
    --font-size-large: 20px; 
}

:root, .light-theme {
    --primary-bg: white;
    ...
}

.dark-theme {
    --primary-bg: black;
    ...
}
```

Theming can be implemented by overring CSS variables in a rule dedicated to the theme.
If you want to apply the theme you just have to add the corresponding class to `<body>`:

```html
<body class="dark-theme">
    ...
    <!-- A theme can also be selectively applied in a specific region. -->
    <div class="my-custom-theme">...</div>
</body>
```

### Property Inference Rules

The inference rules are quite simple. here is a quick overview:
- Names including a CSS property name affect the corresponding CSS property.
  For examples: `--default-border` affects `border`, and `--primary-border-color` affects `border-color`
- `bg` is a shorthand for `background` 
- Names including `foreground` or `fg` affect the `color` property, for example `--primary-fg`
- `text` is an alias for `font`. For example: `--heading-text-size` affects `font-size`

**Background**

- [background-color](#background-color)
- [background-image](#background-image)
- [background and more](#background)

**Font**

- [font-color](#font-color)
- [font-size](#font-size)
- [font-weight](#font-weight)
- [font-family](#font-family)
- [font-style](#font-style)
- [font](#font)


**Border**

- [border-image](#border-image)
- [border and more](#border)

### `background-color`

Variables whose name contains any of the following substrings will be used to
set the value of the `background-color` property.

- `background-color`
- `bg-color`
- `color-bg`

Examples: 
- `--primary-bg-color`

### `background-image`

Variables whose name contains any of the following substrings will be used to
set the value of the `background-image` property.

- `background-image`
- `background-img`
- `bg-img`
- `img-bg`

Examples: 
- `--bg-img-url`

### `background`

Variables whose name contains any of the following substrings will be used to
set the value of the `background` property.

- `background`
- `-bg`
- `bg-`

Examples: 
- `--primary-bg`

⚠️ Variables whose names contains any of the following substrings will not be used
for `background`.

- `(background|bg)-attachment` -> the variable will be used for the
  `background-attachment` property.
- `(background|bg)-blend-mode`
- `(background|bg)-clip`
- `(background|bg)-origin`
- `(background|bg)-position`
- `(background|bg)-size`

### `font-color`

Variables whose name contains any of the following substrings will be used for
to set the value of the `font-color` property.

- `font-color`
- `text-color`
- `foreground`
- `fg-`
- `-fg`

Examples: 
- `--primary-fg`

### `font-size`

Variables whose name contains any of the following substrings will be used to
set the value of the `font-size` property.

- `font-size`
- `text-size`
- `foreground`
- `fs-`
- `-fs`
- `ts-`
- `-ts`

Examples: 
- `--heading-fs`

### `font-weight`

Variables whose name contains any of the following substrings will be used to
set the value of the `font-weight` property.

- `font-weight`
- `text-weight`
- `fw-`
- `-fw`

Examples: 
- `--heading-fw`

### `font-family`

Variables whose name contains `font-family` will be used to
set the value of the `font-family` property.

### `font-style`

Variables whose name contains `font-style` will be used to
set the value of the `font-style` property.

### `font`

Variables whose name contains `font` will be used to
set the value of the `font` property.

### `border-image`

- `border-(image|img)` -> `border-image`
- `border-(image|img)-<prop>` -> `border-image-<prop>`

### `border`

- `border` -> `border`
- `border-color` -> `border-color`
- `border-top-color` -> `border-top-color`

Examples: 
- `--muted-border-color` affects `border-color`
- `--muted-border` affects `border`