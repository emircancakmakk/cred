# Name matching

`cred <project>` and `cred <project> <service> <field>` resolve names in this
order:

1. Exact name.
2. One name that matches ignoring case.
3. One name that starts with the query, ignoring case.
4. Several matches: an error that lists the candidate names. The selector
   does not open.

`add`, `set`, `rm`, and `import` require the stored spelling. A prefix is a
different name there. `import` skips a name that differs only by case from
one already stored.

Case comparison uses Unicode case folding (`strings.EqualFold`). `my-project` and
`MY-PROJECT` match. Turkish `i` / `İ` and `ı` / `I` stay distinct, so `ışık` and
`IŞIK` do not match.

Names must be non-empty, must not start or end with a space, and must not
contain `/`, `\`, or a control character. Spaces inside a name are kept.
Prompted names are stored as typed.


[← README](../README.md) · [Documentation](README.md)
