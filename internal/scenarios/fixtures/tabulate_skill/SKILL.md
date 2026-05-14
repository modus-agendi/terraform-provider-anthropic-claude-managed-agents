---
name: ascii-table-rendering
description: Renders structured key-value facts as an ASCII table using the Python `tabulate` package. Load when the user asks to "render as a table", "format as a table", "make a table of facts", or otherwise present a small set of facts in tabular form.
---

# ASCII Table Rendering with `tabulate`

The `tabulate` Python package is installed in the environment. Use it
whenever you need to present a small set of facts as a readable table.

## Standard idiom

```python
from tabulate import tabulate

rows = [
    ["License", "MIT"],
    ["Language", "Python"],
    ["Main module", "requests"],
]
print(tabulate(rows, headers=["Field", "Value"], tablefmt="grid"))
```

Always use `tablefmt="grid"` for consistent output. The `grid` format
produces clean ASCII borders that are easy for both humans and graders
to read.

## When to use this skill

- The user explicitly asks for "a table" of structured data.
- You have 2–10 short key-value pairs to present and a list is too flat.
- The output will be saved or shared verbatim (memory store, file write,
  message body).

## When NOT to use this skill

- Long-form prose. Tables hurt readability when values are full
  sentences.
- Single-key data. A one-row table is overhead; just say the value.
- Numeric tables with many columns. Use a dataframe library instead.
