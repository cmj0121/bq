# bq

> binary query and modification tool

**bq** is the lightweight and portable command-line tool for binary query and modification tool.
It like `jq` or `yq`, but for binary files, such as images, executables, or any other binary data formats.
Not only can **bq** read binary data, but also can pretty-print it in human-readable format, and modify
binary data
by changing specific fields.

bq is written in Go and can be easily installed on various platforms, including Windows, macOS, and Linux.
You can also cross-compile it for different architectures depending on your needs, or download pre-compiled
binaries.

## Syntax

Like `jq` and `yq`, **bq** uses a simple and expressive syntax for querying and modifying binary data.
It is inspired by [struct][0] module in Python standard library, and using single-character format codes to
represent data types, like `i` for integer, `f` for float, `s` for string, etc.

| Character | Size (bytes) | Description       |
| --------- | ------------ | ----------------- |
| b         | 1            | The signed char   |
| B         | 1            | The unsigned char |
| h         | 2            | The signed short  |
| H         | 2            | The unsigned short|
| i         | 4            | The signed int    |
| I         | 4            | The unsigned int  |
| q         | 8            | The signed long   |
| Q         | 8            | The unsigned long |

Also, using the following prefixes to specify byte order:

- `<` : little-endian
- `>` : big-endian
- `@` : native order in the current system

For example, `<bq` means read the first 9-bits as the little-endian which first byte treated as signed char,
and the next 8 bytes treated as signed long.

In more, you can use digits to specify the number of elements in an array, like `4B` means read 4 unsigned
chars as an array, and `2H` means read 2 unsigned shorts as an array.

### Functions

The `bq` command supports several functions for querying and modifying binary data, that can be combined to
perform complex operations. The following table list the general functions that can evaluate or combine values,
or access the specified fields, even naming as the variables.

| Function     | Description                                          |
| ------------ | ---------------------------------------------------- |
| `parse(...)` | Parse the binary data according to the format string |
| `\|`         | Pipe the left evaluation result to right function    |
| `{...}`      | Create an object by grouping fields                  |

Wihout of the general, the `bq` command evaluate the raw string as the format string to parse the binary data,
like pass the format string to `parse(...)` function. For example, `bH` is equivalent to `parse("bH")`.

### Objects

In general, the `bq` process the input binary data into a list of variables and without names. You can convert
it into the object with named fields by using the `{...}` syntax. You MUST TO provides the same number of fields
names as the number of variables in the input data. For example: `bH | {0 -> key, 1 -> value}` means read a signed
char and an unsigned short, then create an object with fields named `key` and `value` with the specified values.
You also can create nested objects by using the same syntax, like `bHQ | {0 -> header, {1 -> data, 2 -> footer}}`.

[0]: https://docs.python.org/3.14/library/struct.html
