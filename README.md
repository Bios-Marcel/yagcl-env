# YAGCL env

[![Go Reference](https://pkg.go.dev/badge/github.com/Bios-Marcel/yagcl-env.svg)](https://pkg.go.dev/github.com/Bios-Marcel/yagcl-env)
[![Build and Tests](https://github.com/Bios-Marcel/yagcl-env/actions/workflows/test.yml/badge.svg?branch=master)](https://github.com/Bios-Marcel/yagcl-env/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/Bios-Marcel/yagcl-env/branch/master/graph/badge.svg?token=82SUL3UD8H)](https://codecov.io/gh/Bios-Marcel/yagcl-env)

This package provides a [YAGCL](https://github.com/Bios-Marcel/yagcl)
[source](https://pkg.go.dev/github.com/Bios-Marcel/yagcl#Source) that provides
access to reading environment variables.

## Usage

```
go get github.com/Bios-Marcel/yagcl-env
```

## Reporting Issues / Requesting features

All "official" sources for YAGCL should be reported in the [main repositories
issue section](https://github.com/Bios-Marcel/yagcl/issues).

## Syntax

### Reserved characters

Reserved characters have a concrete meaning for certain value types.
Each value type can have different reserved characters. Check the documentation
for the corresponding type in order to see them.

Each reserved character can be escaped via `\`.
If you need a literal `\`, write `\\` instead.

### Arrays / Slices

These types support lists of 0 to N elements. If you have a fixed size
array, you'll need to supply an exact amount of elements.

The elements can be of any type, as long as the type is parsable.

If your type is `[]string`, the elements will be conveterted into `string`, if
you have an `[]int`, you'll have to pass only valid `int` values. Values are
separated by single commas.

For example, if given:

```env
VAR_NAME=word,12,Hello\, Chris
```

You would get:

```json
["word","12","Hello, Christ"]
```

Reserved characters:
* `,`

### Maps

This type allows you to do a `KEY=VALUE` mapping, where you can have more than
one key-value pair.

The pairs are separated by single commas. The key and the value are separated by an equal sign.

For example, if given:

```env
VAR_NAME=a=1,b=2,weird\=key=3
```

You would get:

```json
{
    "a": 1,
    "b": 2,
    "weird=key": 3,
}
```

Reserved characters:
* `,`
* `=`
