# bq

> binary query and modification tool

**bq** is the lightweight and portable command-line tool for binary query and modification tool.
It like `jq` or `yq`, but for binary files, such as images, executables, or any other binary data formats.
Not only can **bq** read binary data, but also can pretty-print it in human-readable format, and modify
binary data by changing specific fields.

bq is written in Go and can be easily installed on various platforms, including Windows, macOS, and Linux.
You can also cross-compile it for different architectures depending on your needs, or download pre-compiled
binaries.

## Installation

```bash
go install github.com/cmj0121/bq/cmd/bq@latest
```

Or build from source:

```bash
git clone https://github.com/cmj0121/bq.git
cd bq
go build -o bq ./cmd/bq
```

## Usage

```bash
# Read binary data with format codes
printf '\xff\x01\x02' | bq '<bH'

# Pretty print with -p flag
printf '\xff\x01\x02' | bq '<bH' -p

# Read arrays with count prefix
printf '\x01\x02\x03\x04' | bq '4B' -p

# Create named objects with pipe operator
printf '\xff\x01\x02' | bq '<bH | {0 -> key, 1 -> value}' -p
```

## Syntax

Like `jq` and `yq`, **bq** uses a simple and expressive syntax for querying and modifying binary data.
It is inspired by [struct][0] module in Python standard library, and using single-character format codes to
represent data types.

### Format Codes

| Character | Size (bytes) | Go Type  | Description        |
| --------- | ------------ | -------- | ------------------ |
| b         | 1            | int8     | The signed char    |
| B         | 1            | uint8    | The unsigned char  |
| h         | 2            | int16    | The signed short   |
| H         | 2            | uint16   | The unsigned short |
| i         | 4            | int32    | The signed int     |
| I         | 4            | uint32   | The unsigned int   |
| q         | 8            | int64    | The signed long    |
| Q         | 8            | uint64   | The unsigned long  |

### Byte Order Prefixes

| Prefix | Description                      |
| ------ | -------------------------------- |
| `<`    | Little-endian                    |
| `>`    | Big-endian                       |
| `@`    | Native order (system default)    |

**Example:** `<bq` means read in little-endian: first byte as signed char, next 8 bytes as signed long.

### Arrays

Use digit prefix to read multiple elements as an array:

| Expression | Description                             | Result Type   |
| ---------- | --------------------------------------- | ------------- |
| `4B`       | Read 4 unsigned chars                   | []uint8       |
| `2H`       | Read 2 unsigned shorts                  | []uint16      |
| `<b4B`     | Read 1 signed char + 4 unsigned chars   | int8, []uint8 |

**Example:**

```bash
$ printf '\x01\x02\x03\x04' | bq '4B' -p
Name       Code   Type                    Value                  Hex
--------------------------------------------------------------------
0          B      uint8                                [01 02 03 04]
```

### Functions

#### parse()

The `parse()` function provides explicit parsing syntax:

```text
parse(<format_codes>)
```

This is equivalent to using format codes directly, but provides a more explicit syntax:

```bash
# These are equivalent:
printf '\xff\x01\x02' | bq '<bH' -p
printf '\xff\x01\x02' | bq 'parse(<bH)' -p
```

### Pipe Operator

The pipe operator `|` passes parsed values to subsequent operations:

```text
<format_codes> | <operation>
```

### Objects

Convert parsed values into named fields using the object syntax `{...}`:

```text
<format_codes> | {<index> -> <name>, ...}
```

**Syntax:**

- `<index>` - zero-based index into parsed values
- `<name>` - field name

**Example:**

```bash
$ printf '\xff\x01\x02' | bq '<bH | {0 -> header, 1 -> length}' -p
Name       Code   Type                    Value                  Hex
--------------------------------------------------------------------
header     b      int8                       -1                 0xff
length     H      uint16                    513               0x0201
```

### Nested Objects

Create hierarchical structures using the nested object syntax `<name>: {...}`:

```text
<format_codes> | {<index> -> <name>, <nested_name>: {<index> -> <name>, ...}}
```

**Syntax:**

- `<nested_name>:` - nested object field name followed by colon
- `{...}` - nested object containing field definitions

**Example:**

```bash
$ printf '\xff\x01\x02\x03' | bq '<bHB | {0 -> header, nested: {1 -> length, 2 -> flag}}' -p
Name       Code   Type                    Value                  Hex
--------------------------------------------------------------------
header     b      int8                       -1                 0xff
nested     -      object
  length   H      uint16                    513               0x0201
  flag     B      uint8                       3                 0x03
```

Nested objects can be arbitrarily deep:

```bash
$ printf '\x01\x02\x00\x03\x00\x00\x00' | bq '<bHi | {0 -> a, level1: {1 -> b, level2: {2 -> c}}}' -p
Name       Code   Type                    Value                  Hex
--------------------------------------------------------------------
a          b      int8                        1                 0x01
level1     -      object
  b        H      uint16                      2               0x0002
  level2   -      object
    c      i      int32                       3           0x00000003
```

### Combined Example

Reading a binary header with magic bytes and a length field:

```bash
$ printf '\x89PNG\x00\x00\x00\x0d' | bq '<4Bi | {0 -> magic, 1 -> chunk_length}' -p
Name       Code   Type                    Value                  Hex
--------------------------------------------------------------------
magic      B      []uint8                              [89 50 4e 47]
chunk_length i    int32               218103808           0x0d000000
```

## Flags

| Flag | Description                              |
| ---- | ---------------------------------------- |
| `-p` | Pretty print output in table format      |
| `-v` | Increase verbosity (use multiple times)  |
| `-f` | Input file (default: stdin with `-`)     |

## Roadmap

- [x] `parse(...)` function for explicit parsing
- [x] Nested objects: `{0 -> a, nested: {1 -> b, 2 -> c}}`
- [ ] Write/modify binary data
- [ ] String type support (`s`)
- [ ] Float type support (`f`, `d`)

[0]: https://docs.python.org/3.14/library/struct.html
