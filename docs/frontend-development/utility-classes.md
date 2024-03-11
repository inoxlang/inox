# Utility Classes

Inox comes with a subset of [Tailwind](https://tailwindcss.com/) and a utility class system based on CSS variables (custom properties).

- The provided Tailwind's subset is relatively small: **you don't need to read the official documentation and no configuration is required by default.** An optional configuration system will likely be supported in the future.
- Theming is implemented using CSS variables, not Tailwind.
- Used utility classes are automatically added to the file `/static/css/utility-classes.css`.

- [Tailwind](#tailwind)
    - [Breakpoint modifiers](#breakpoint-modifiers)

- [Variable-Based Utilities](#variable-based-utilities)
    - [Variable definitions and theming](#variable-definitions-and-theming)
    - [background-color](#background-color)
    - [background-image](#background-image)
    - [background and more](#background)
    - [font-color](#font-color)
    - [font-size](#font-size)
    - [font-weight](#font-weight)

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
So it adds a rule to `/static/css/utility-classes.css`:

```css
.--primary-bg {
    background: var(--primary-bg);
}
```

Here is the list of supported CSS properties:

- [background-color](#background-color)
- [background-image](#background-image)
- [background and more](#background)
- [font-color](#font-color)
- [font-size](#font-size)
- [font-weight](#font-weight)

**More properties will be supported.**


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
    <!-- A theme can also be selectively applied in a specic region. -->
    <div class="my-custom-theme">...</div>
</body>
```

### `background-color`

Variables whose name contains any of the following substrings will be used to
set the value of the `background-color` property.

- `background-color`
- `bg-color`
- `color-bg`
- `color-bg`

### `background-image`

Variables whose name contains any of the following substrings will be used to
set the value of the `background-image` property.

- `background-image`
- `background-img`
- `bg-img`
- `img-bg`

### `background`

Variables whose name contains any of the following substrings will be used to
set the value of the `background` property.

- `background`
- `-bg`
- `bg-`

⚠️ Variables whose names contains any of the following substrings will not be used
for `background`.

- `(background|bg)-attachment` -> the variable will be used for the
  `background-attachment` property.
- `(background|bg)-blend-mode`
- `(background|bg)-clip`
- `(background|bg)-origin`
- `(background|bg)-position`
- `(background|bg)-size`
- `(background|bg)-size`

### `font-color`

Variables whose name contains any of the following substrings will be used for
to set the value of the `font-color` property.

- `font-color`
- `text-color`
- `foreground`
- `fg-`
- `-fg`

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

### `font-weight`

Variables whose name contains any of the following substrings will be used to
set the value of the `font-weight` property.

- `font-weight`
- `text-weight`
- `fw-`
- `-fw`
