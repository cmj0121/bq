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

[0]: https://docs.python.org/3.14/library/struct.html
