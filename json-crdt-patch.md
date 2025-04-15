---

# JSON CRDT Patch

*JSON CRDT Patch* specification defines how changes
to [JSON CRDT][json-crdt] documents are described. This specification
lays out the semantics and encoding of JSON CRDT Patch documents.

[json-crdt]: /specs/json-crdt


---

# JSON CRDT Patch > Patch document

A JSON CRDT Patch document is the smallest atomic unit of change for JSON CRDT
documents. This section describes the semantics of JSON CRDT Patch documents.

~~~jj.note
#### Relation to JSON Patch

JSON CRDT Patch is similar to [JSON Patch][json-patch], in the sense that
both are patch protocols for JSON document changes. However, JSON Patch is
designed for regular JSON documents, while JSON CRDT Patch specifies change
semantics for JSON CRDT documents.

[json-patch]: https://datatracker.ietf.org/doc/html/rfc6902
~~~


---

# JSON CRDT Patch > Patch document > Logical Clock

JSON CRDT and JSON CRDT Patch documents use a *logical clock* to identify nodes,
to order concurrent edits, and to identify various data entities used in the
CRDT algorithms. Essentially, everything that needs to be identified or
timestamped in a document is assigned a logical timestamp.

A *logical timestamp* is a globally unique identifier, which can be partially
ordered. A logical timestamp is a tuple of two values: (1) a session ID; and
(2) a sequence counter.

A *session ID* (also known as a site ID or a process ID) is a globally unique
identifier, which is (usually randomly) assigned to each client, which
participates in the editing of a JSON CRDT document. Each session ID is a 53-bit
positive integer, which is assumed to be unique across all clients. The first
65,535 (`0xffff`) session IDs are reserved for internal use, hence the possible
session IDs assigned to clients are in the inclusive range of 65,536 to
9,007,199,254,740,991.

A *sequence counter* is a non-negative integer, which is incremented each time
a new logical timestamp is generated for a given session ID or when a clock with
a higher sequence number is observed. The sequence number is advanced according
to the following rules:

1. The sequence number is incremented by one for each new logical timestamp
   generated for a given session ID locally.
2. The sequence number jumps to the maximum value of the sequence number
   observed for any other logical timestamp received from remote clients.

Logical timestamps are compared by comparing their sequence numbers first, and
if the sequence numbers are equal, then the session IDs are compared. The
logical timestamp with the higher sequence number is considered to be the higher
(more recent) one. If the sequence numbers are equal, then the logical timestamp
with the higher session ID is considered to be the more recent one.

In this document logical timestamps are formatted as two integers separated by
a dot, for example, `123.456`. The first integer is the session ID---123---and
the second integer is the sequence number---456.


---

# JSON CRDT Patch > Patch document > Patch Structure

JSON CRDT Patch is an atomic unit of change for JSON CRDT documents. A
patch is applied as a whole---either all operations in the patch are applied, or
none of them.

Each operation is a change to a single JSON CRDT node. An operation can either
create a new JSON CRDT node or update an existing one. The operations are
applied to the document in the order they appear in the patch.

Each patch consists of a header and of one or more operations. The *header*
stores the patch ID and optional custom metadata. The *patch ID* is a logical
clock timestamp, which is used to identify the patch.

```
+-----------------+  -----------------
| Patch ID        |             Header
+-----------------+
| Metadata        |
+-----------------+  -----------------
| Operation 1     |         Operations
+-----------------+
| Operation 2     |
+-----------------+
| ...             |
+-----------------+
```

The ID of the first operation in the patch is equal to the patch ID. The ID of
each subsequent operation is equal to the ID of the previous operation
incremented by the number of clock cycles the previous operation consumes. The
amount of clock cycles consumed by each operation is called the
operation *span*.

The session ID of all operations in the patch is constant, only the logical
clock sequence number changes.

~~~jj.note
This implies that all operations in a single patch are always created by the
same session, i.e. the same user.
~~~


---

# JSON CRDT Patch > Patch document > Node Types

JSON CRDT Patch supports operations on CRDT node types described in JSON CRDT
specification. The following nodes are supported:

- `con` --- a constant value.
- `val` --- a LWW-Value.
- `obj` --- a LWW-Object.
- `vec` --- a LWW-Vector.
- `str` --- an RGA-String.
- `bin` --- an RGA-Binary blob.
- `arr` --- an RGA-Array.


### The `con` Constant

The `con` node type is a constant value, which does not accept any operations
and cannot be changed. The constant value is set at the time of creation of
the `con` node and never changes after that. The only way to change the
value of the `con` node is to delete it from a parent container, such
as an object or an array, and create a new one.

The value of a `con` node is JSON-like value. The value can be any JSON
value, including `null`, `true`, `false`, numbers, strings, arrays, objects,
binary blobs, `undefined` value, and logical clock timestamps.

The only operation supported by the `con` node is `new_con`, which creates
a new `con` instance with a specified value.


### The `val` LWW-Value

The `val` node type is a LWW-Value (Last-Write-Wins Value), which stores a
single value. The value is a reference to another CRDT node. Often it is
a reference to a `con` node.

The `val` nodes support the following operations:

- `new_val` - creates a new `val` node instance.
- `ins_val` - updates the value of a `val` node. The update succeeds
  only if the ID of the new value is greater than the ID of the current value.

A `val` node cannot be deleted, to delete it, it must be removed from
the parent container object, such as `obj`, `vec`, `arr`, or another `val`.


### The `obj` LWW-Object

The `obj` node type is a LWW-Object (Last-Write-Wins Object), which stores a
set of key-value pairs. Each key-value pair is a distinct LWW-Value CRDT object.
The keys are strings, the values are references to other CRDT objects.

The `obj` node type supports the following operations:

- `new_obj` - creates a new `obj` node instance.
- `ins_obj` - inserts or updates key-value pairs in an `obj` node. Each
  key-value pair is a separate LWW register. The update succeeds only if the
  ID of the new value is greater than the ID of the current value.

To delete a key-value pair from an `obj` node, the key must be set
to `con` object with `undefined` value.


### The `vec` LWW-Vector

The `vec` LWW-Vector node type is similar to `obj` LWW-Object. Just like
the `obj` node type, the `vec` store a set of key-value pairs. However, the
keys in `vec` are integers, which start from zero and increment by one for
each new key-value pair. The maximum key value is limited to 255, the minimum
key value is zero.

The `vec` node type supports the following operations:

- `new_vec` - creates a new `vec` node instance.
- `ins_vec` - inserts or updates key-value pairs in a `vec` object. Each
  key-value pair is a separate LWW register. The update succeeds only if the
  ID of the new value is greater than the ID of the current value.

Usually, the `vec` node type is used to store fixed-length arrays, i.e. tuples.
Hence, usually the elements of a `vec` node are never deleted. However, it is
possible to delete elements from a `vec` node by setting the value of the
element to `con` object with `undefined` value.


### The `str` RGA-String

The `str` node type is a RGA-String (Replicated Growable Array String), which
represents a UTF-16 string. The `str` data type represents an ordered list of
UTF-16 code points. A unit of insertion and deletion is a single UTF-16 code
point.

The `str` node type supports the following operations:

- `new_str` - creates a new `str` node instance.
- `ins_str` - inserts a sub-string into a `str` node. The string is inserted
  at a specified position in the `str` object according to the RGA algorithm.
- `del` - deletes a sub-string from a `str` object.


### The `bin` RGA-Binary

The `bin` node type is a RGA-Binary (Replicated Growable Array Binary), which
represents an ordered list of octets. The `bin` node type is similar to
the `str` node type, except that a unit of insertion and deletion is an octet
(8-bit byte), not a character.

The `bin` node type supports the following operations:

- `new_bin` - creates a new `bin` node instance..
- `ins_bin` - inserts a chunk of binary data into a `bin` object. The chunk is
  inserted at a specified position in the `bin` object according to the RGA
  algorithm.
- `del` - deletes a chunk of binary data from a `bin` object.


### The `arr` RGA-Array

The `arr` node type is a RGA-Array (Replicated Growable Array), which
represents an ordered list of CRDT objects. The `arr` type is similar
to the `str` and `bin` types, except that a unit of insertion and
deletion is a reference to another CRDT object, not a character or an octet.

The `arr` node type supports the following operations:

- `new_arr` - creates a new `arr` node instance.
- `ins_arr` - inserts one or more elements into an `arr` object following the
  RGA algorithm.
- `del` - deletes elements from an `arr` object.

The `arr` elements are immutable. To update an element, it must be deleted
and a new element must be inserted in its place. Alternatively, the element
can point to a mutable `val` node, which can be updated in-place.


---

# JSON CRDT Patch > Patch document > Operation Types

JSON CRDT Patch classifies all patch operations into four broad kinds:

- `new` - creates a new CRDT node.
- `ins` - updates an existing CRDT node.
- `del` - deletes contents from and existing CRDT node.
- `nop` - *noop* operation, operation which does nothing.


## The `new` Operation Type

The `new` operations create new CRDT objects. Usually, they have no other
payload. An exception is the `con` node. The `con` nodes are
created with a payload, which is the initial value of the node.


## The `ins` Operation Type

The `ins` operations update existing CRDT nodes. The payload and semantics of
the update depend on the type of the CRDT node. All `ins` operations reference
the CRDT node they update by its ID.


## The `del` Operation Type

The `del` operations delete contents of existing list CRDT objects.
All `del` operations reference the ID of the CRDT node they delete contents
from. The `del` operations are uniform --- they have the same payload and
semantics for all list CRDT nodes.


## The `nop` Operation Type

The `nop` operations do nothing. They are used to skip over logical clock cycles
in a patch.

JSON CRDT Patch encodings do not store IDs for each operation. Instead, only the
starting ID of the patch is stored. All subsequent IDs are calculated by
incrementing the previous ID by the number of clock cycles the operation takes.
The `nop` operation allows to skip over clock cycles.


---

# JSON CRDT Patch > Patch document > Operations

Each JSON CRDT Patch consists of one or more operations. An operation is an
immutable unit of change. Each operation has a unique ID, which is a logical
timestamp. Operations might also have a reference to some CRDT node and
a payload.

In total, JSON CRDT Patch defines 15 operations. There are seven operations that
create new CRDT nodes:

- `new_con` --- creates a new `con` node.
- `new_val` --- creates a new `val` node.
- `new_obj` --- creates a new `obj` node.
- `new_vec` --- creates a new `vec` node.
- `new_str` --- creates a new `str` node.
- `new_bin` --- creates a new `bin` node.
- `new_arr` --- creates a new `arr` node.

There are seven operations that update existing CRDT nodes:

- `ins_val` --- updates value of a `val` node.
- `ins_obj` --- inserts or updates key-value pairs of an `obj` node.
- `ins_vec` --- inserts or updates elements of a `vec` node.
- `ins_str` --- inserts text contents into a `str` node.
- `ins_bin` --- inserts binary contents into a `bin` node.
- `ins_arr` --- inserts elements into an `arr` node.
- `del` --- deletes contents from list CRDT nodes (`str`, `bin`, and `arr`).

And there is one operation that does nothing:

- `nop` --- does nothing.


## Operation List


### The `new_con` Operation

The `new_con` operation creates a new `con` node. The operation has an ID,
which is implicitly computed from the position in the patch.

The payload of the `new_con` operation is the value of the `con` object, which
can be one of the following three options:

1. Any JSON (or subset of CBOR) value, including: `null`, booleans, numbers,
   strings, arrays, objects, and binary blobs.
2. An `undefined` value, which indicates that the `con` object is empty.
3. A logical timestamp, which is a 2-tuple of integers.

~~~jj.note
JSON CRDT supports binary data. When binary data is used inside a `con` object,
care needs to be taken to ensure that the patch is encoded in a way that
supports binary data.

When `binary` encoding is used, it will automatically encode the binary data
correctly. When `compact` or `verbose` encoding is used, one needs to use a JSON
encoding format that supports binary data, such as CBOR or MessagePack.
~~~

~~~jj.note
The `undefined` and logical clock values can only appear at the root of the
`con` object. They cannot appear inside nested `con` objects. As a result,
all JSON CRDT Patch encodings supports those values without placing any
addition restrictions on the `compact` and `verbose` encoding serialization.
~~~

The `new_con` operation consists of the following parts:

- `new_con.id` --- the ID of the operation, a logical timestamp.
- `new_con.value` --- the raw data of the node, any JSON/CBOR value, or
  `undefined`, or a logical clock value.

The operation consumes one logical clock cycle, its span is 1.


### The `new_val` Operation

The `new_val` operation creates a new `val` node. The operation has an ID,
which is implicitly computed from the position in the patch.

The `new_val` operation consists of the following parts:

- `new_val.id` --- the ID of the operation, a logical timestamp.

The operation consumes one logical clock cycle, its span is 1.


### The `new_obj` Operation

The `new_obj` creates a new `obj` node. The operation has an ID, which is
implicitly computed from the position in the patch.

The `new_obj` operation consists of the following parts:

- `new_obj.id` --- the ID of the operation, a logical timestamp.

The operation consumes one logical clock cycle, its span is 1.


### The `new_vec` Operation

The `new_vec` creates a new `vec` node. The operation has an ID, which is
implicitly computed from the position in the patch.

The `new_vec` operation consists of the following parts:

- `new_vec.id` --- the ID of the operation, a logical timestamp.

The operation consumes one logical clock cycle, its span is 1.


### The `new_str` Operation

The `new_str` creates a new `str` node. The operation has an ID, which is
implicitly computed from the position in the patch. The operation consumes one
logical clock cycle.

The `new_str` operation consists of the following parts:

- `new_str.id` --- the ID of the operation, a logical timestamp.


### The `new_bin` Operation

The `new_bin` creates a new `bin` object. The operation has an ID, which is
implicitly computed from the position in the patch.

The `new_bin` operation consists of the following parts:

- `new_bin.id` --- the ID of the operation, a logical timestamp.

The operation consumes one logical clock cycle, its span is 1.


### The `new_arr` Operation

The `new_arr` creates a new `arr` node. The operation has an ID, which is
implicitly computed from the position in the patch.

The `new_arr` operation consists of the following parts:

- `new_arr.id` --- the ID of the operation, a logical timestamp.

The operation consumes one logical clock cycle, its span is 1.


### The `ins_val` Operation

The `ins_val` operation updates the value of a `val` node. The operation has
an ID, which is implicitly computed from the position in the patch.

The `ins_val` operation consists of the following parts:

- `ins_val.id` --- the ID of the operation, a logical timestamp.
- `ins_val.node` --- the ID of the `val` node to be updated.
- `ins_val.value` --- the new value of the `val` node, a logical timestamp,
  a reference to another JSON CRDT node.

The operation consumes one logical clock cycle, its span is 1.


### The `ins_obj` Operation

The `ins_obj` operation inserts or updates key-value pairs of an `obj` node.
The operation has an ID, which is implicitly computed from the position in the
patch.

The `ins_obj` operation consists of the following parts:

- `ins_obj.id` --- the ID of the operation, a logical timestamp.
- `ins_obj.node` --- the ID of the `obj` node to be updated.
- `ins_obj.map` --- a map of key-value pairs to be inserted or updated. Each key
  is a string, and each value is a logical timestamp, a reference to another
  JSON CRDT node.

The operation consumes one logical clock cycle, its span is 1.


### The `ins_vec` Operation

The `ins_vec` operation inserts or updates elements of a `vec` node. The
operation has an ID, which is implicitly computed from the position in the
patch.

The `ins_vec` operation consists of the following parts:

- `ins_vec.id` --- the ID of the operation, a logical timestamp.
- `ins_vec.node` --- the ID of the `vec` node to be updated.
- `ins_vec.map` --- a map of index-value pairs to be inserted or updated. Each
  index is a non-negative integer, and each value is a logical timestamp, a
  reference to another JSON CRDT node.

The operation consumes one logical clock cycle, its span is 1.


### The `ins_str` Operation

The `ins_str` operation inserts text contents into a `str` node. The
operation has an ID, which is implicitly computed from the position in the
patch.

The `ins_str` operation consists of the following parts:

- `ins_str.id` --- the ID of the operation, a logical timestamp.
- `ins_str.node` --- the ID of the `str` node to be updated.
- `ins_str.ref` --- ID of the element after which the text is inserted.
- `ins_str.data` --- the text to be inserted.

The operation consumes the number of clock cycles equal to the length of the
inserted text, which is the length of text in UTF-16 code units.


### The `ins_bin` Operation

The `ins_bin` operation inserts binary contents into a `bin` node. The
operation has an ID, which is implicitly computed from the position in the
patch.

The `ins_bin` operation consists of the following parts:

- `ins_bin.id` --- the ID of the operation, a logical timestamp.
- `ins_bin.node` --- the ID of the `bin` node to be updated.
- `ins_bin.ref` --- ID of the element after which new elements are inserted.
- `ins_bin.data` --- the binary contents to be inserted.

The operation consumes the number of clock cycles equal to the length of octets
in the inserted binary contents---its span is equal to the byte length of the
`ins_bin.data` blob.


### The `ins_arr` Operation

The `ins_arr` operation inserts elements into an `arr` node. The operation
has an ID, which is implicitly computed from the position in the patch.

The `ins_arr` operation consists of the following parts:

- `ins_arr.id` --- the ID of the operation, a logical timestamp.
- `ins_arr.node` --- the ID of the `arr` node to be updated.
- `ins_arr.ref` --- ID of the element after which new elements are inserted.
- `ins_arr.data` --- the list of elements to be inserted.

The operation consumes the number of clock cycles equal to the number of
inserted elements---its span is the length of the `ins_arr.data` list.


### The `del` Operation

The `del` operation deletes contents from RGA ordered list objects, such as
`str`, `bin`, and `arr` nodes. The operation has an ID, which is implicitly
computed from the position in the patch.

The `del` operation consists of the following parts:

- `del.id` --- the ID of the operation, a logical timestamp.
- `del.node` --- the ID of the RGA node to be updated.
- `del.list` --- the list of ID ranges to be deleted.

The operation consumes one logical clock cycle, its span is 1.


### The `nop` Operation

The `nop` operation is a no-op operation, it does nothing. The operation has
an ID, which is implicitly computed from the position in the patch.

The payload of the `nop` operation is the number of logical clock cycles to
advance. Hence, the operation consumes the specified number of logical clock
cycles.


---

# JSON CRDT Patch > Patch document > Operation Naming

Usually, JSON CRDT Patch operations are stored as a flat list of operations. We
need to be able to identify each operation in the list. We do that by using
a mnemonic in text-based encodings, and an opcode in binary encodings.


## Mnemonics

Below table lists all mnemonics:

```
+===================================================================+
| Mnemonics                                                         |
+===================================================================+
|           | new         | ins         | del         | nop         |
+-----------+-------------+-------------+-------------+-------------+
| con       | new_con     |             |             |             |
| val       | new_val     | ins_val     |             |             |
| obj       | new_obj     | ins_obj     |             |             |
| vec       | new_vec     | ins_vec     |             |             |
| str       | new_str     | ins_str     | del         |             |
| bin       | new_bin     | ins_bin     | del         |             |
| arr       | new_arr     | ins_arr     | del         |             |
| ø         |             |             |             | nop         |
+-----------+-------------+-------------+-------------+-------------+
```


## Opcodes

~~~jj.aside
A useful property of opcodes is that they all are no greater than 23. This means
that they can be encoded in a single byte in CBOR and MessagePack encodings.
Also, they can fit into 5 bits in binary encoding, which leaves the other 3 bits
of an octet for other information.
~~~

In binary encoding, each operation is identified by a 5-bit opcode.
Below table lists all opcodes, the opcodes are represented in binary and
decimal values in parentheses:

```
+===================================================================+
| Opcodes                                                           |
+===================================================================+
|           | new         | ins         | del         | nop         |
+-----------+-------------+-------------+-------------+-------------+
| con       | 00_000 (0)  |             |             |             |
| val       | 00_001 (1)  | 01_001 (9)  |             |             |
| obj       | 00_010 (2)  | 01_010 (10) |             |             |
| vec       | 00_011 (3)  | 01_011 (11) |             |             |
| str       | 00_100 (4)  | 01_100 (12) | 10_000 (16) |             |
| bin       | 00_101 (5)  | 01_101 (13) | 10_000 (16) |             |
| arr       | 00_110 (6)  | 01_110 (14) | 10_000 (16) |             |
| ø         |             |             |             | 10_001 (17) |
+-----------+-------------+-------------+-------------+-------------+
```


## Operation Summary Table

Below table summarizes the operations defined in the JSON CRDT Patch.

~~~jj.wide {rightOnly: true}
```
+===================================================================================================+
| Operations                                                                                        |
|==========================+======================+=============+===================================+
|                          | Naming               | Opcode      | Contents                          |
|                          +-----+-----+----------+-------+-----+-----+------------------+----------+
|                          | Op  | Obj | Mnemonic | Bin   | Dec | Obj | Payload          | Span     |
|                          +-----+-----+----------+-------+-----+-----+------------------+----------+
| 1   Make Const           | new | con | new_con  | 00000 | 0   | No  | Value            | 1        |
| 2   Make LWW-Value       | new | val | new_val  | 00001 | 1   | No  | No               | 1        |
| 3   Make LWW-Object      | new | obj | new_obj  | 00010 | 2   | No  | No               | 1        |
| 4   Make LWW-Vector      | new | vec | new_vec  | 00011 | 3   | No  | No               | 1        |
| 5   Make RGA-String      | new | str | new_str  | 00100 | 4   | No  | No               | 1        |
| 6   Make RGA-Binary      | new | bin | new_bin  | 00101 | 5   | No  | No               | 1        |
| 7   Make RGA-Array       | new | arr | new_arr  | 00110 | 6   | No  | No               | 1        |
| 8   Update LWW-Value     | ins | val | ins_val  | 01001 | 9   | Yes | New value ID     | 1        |
| 9   Update LWW-Object    | ins | obj | ins_obj  | 01010 | 10  | Yes | List of tuples   | 1        |
| 10  Update LWW-Vector    | ins | vec | ins_vec  | 01011 | 11  | Yes | List of tuple    | 1        |
| 11  Insert in RGA-String | ins | str | ins_str  | 01100 | 12  | Yes | After ID, string | Str len  |
| 12  Insert in RGA-Binary | ins | bin | ins_bin  | 01101 | 13  | Yes | After ID, blob   | Blob len |
| 13  Insert in RGA-Array  | ins | arr | ins_arr  | 01110 | 14  | Yes | After ID, IDs    | List len |
| 14  Delete from RGA      | del | *   | del      | 10000 | 16  | Yes | List of spans    | 1        |
| 15  No-op                | nop | ø   | nop      | 10001 | 17  | No  | Length           | 1+       |
+---------------------------------------------------------------------------------------------------+
```
~~~


---

# JSON CRDT Patch > Encoding

The JSON CRDT Patch can be serialized into different formats. The following
formats are supported:

- `verbose` - a verbose human-readable JSON encoding.
- `compact` - a JSON encoding which follows Compact JSON encoding scheme.
- `binary` - a custom designed minimal binary encoding.


---

# JSON CRDT Patch > Encoding > Verbose Format

The `verbose` encoding format specifies how JSON CRDT Patch is serialized into
a human-readable JSON-like objects.

The resulting patch does not have to be encoded as JSON, other JSON-like
serializers---such as CBOR or MessagePack---can be used as well.

The patch is represented as a JSON object with the following fields:

- `id` - the ID of the patch. The ID (logical timestamp) of the first operation
  of the patch. `id` is encoded as an array 2-tuple, where first element is the
  session ID and the second element is the  logical time sequence number.
- `meta` - optional, the metadata of the patch. This can be any valid JSON
  object specified by the application.
- `ops` - the list of operations in the patch. The list is encoded as an array
  of JSON objects, where each object represents an operation. The ID of each
  operation is implicitly computed by adding the logical time span of the
  previous operation to the ID of the previous operation. The first operation
  of the patch has an ID equal to the ID of the patch.

The following example shows a patch with two operations:

```json
{
  "id": [123, 456],
  "meta": { "author": "John Doe" },
  "ops": [
    { "op": "new_obj" },
    { "op": "new_str" },
  ],
}
```

Patch operations are encoded as JSON objects. Each operation has an `op` field,
which is a string equal to the operation mnemonic. Operations, which reference
other objects, have an `obj` field, which is and ID of the referenced object.
Operations, which have payload, have a `value` field, which is the payload of
the operation. There maybe be other fields, which are specific to the
operation, see the operation definitions for details.

ID (logical timestamps) can be encoded in one of the following ways:

- As an array 2-tuple, where first element is the session ID and the second
  element is the logical time sequence number.
- If the session ID is equal to the session ID of the patch, then the ID can
  be encoded as a single number, which is the logical time sequence number
  difference between the ID and the ID of the patch.


## `new_con` Operation Encoding

The `new_con` operation is encoded as a JSON object with the `"op"` field
equal to `"new_con"` and `"value"` field equal to the value of the constant.

```json
{ "op": "new_con", "value": 42 }
```

A constant with `undefined` value is encoded by omitting the `"value"` field.

```json
{ "op": "new_con" }
```

A logical timestamp is encoded by setting the `"timestamp"` field to `true`.

```json
{ "op": "new_con", "timestamp": true, "value": [123, 456] }
```

If timestamp session ID is equal to the session ID of the patch, then the
timestamp can be encoded as a single number, which is the logical time sequence
number difference between the timestamp and the ID of the patch.

```json
{ "op": "new_con", "timestamp": 456, "value": 0 }
```


## `new_val` Operation Encoding

The `new_val` operation is encoded as a JSON object with the `"op"` field equal
to `"new_val"`,

```json
{ "op": "new_val" }
```


## `new_obj` Operation Encoding

The `new_obj` operation is encoded as a JSON object with the `"op"` field equal
to `"new_obj"`.

```json
{ "op": "new_obj" }
```


## `new_vec` Operation Encoding

The `new_vec` operation is encoded as a JSON object with the `"op"` field
equal to `"new_vec"`.

```json
{ "op": "new_vec" }
```


## `new_str` Operation Encoding

The `new_str` operation is encoded as a JSON object with the `"op"` field
equal to `"new_str"`.

```json
{ "op": "new_str" }
```


## `new_bin` Operation Encoding

The `new_bin` operation is encoded as a JSON object with the `"op"` field
equal to `"new_bin"`.

```json
{ "op": "new_bin" }
```


## `new_arr` Operation Encoding

The `new_arr` operation is encoded as a JSON object with the `"op"` field
equal to `"new_arr"`.

```json
{ "op": "new_arr" }
```


## `ins_val` Operation Encoding

The `ins_val` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_val"`.
- `"obj"` --- the ID of the `val` object, which value is updated.
- `"value"` --- the ID (logical timestamp) of the new value.

For example:

```json
{ "op": "ins_val", "obj": [123, 0], "value": [123, 1] }
```


## `ins_obj` Operation Encoding

The `ins_obj` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_obj"`.
- `"obj"` --- the ID of the `obj` object, which value is updated.
- `"value"` --- an array of 2-tuples, where each 2-tuple is a field name
  and an ID (logical timestamp) of the field value.

An example where all timestamps are encoded as 2-tuples:

```json
{
  "op": "ins_obj",
  "obj": [123, 0],
  "value": [
    ["foo", [123, 1]],
    ["bar", [123, 2]]
  ]
}
```

Same example, but with timestamps encoded as time differences:

```json
{
  "op": "ins_obj",
  "obj": 1,
  "value": [
    ["foo", 2],
    ["bar", 3]
  ]
}
```


## `ins_vec` Operation Encoding

The `ins_vec` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_vec"`.
- `"obj"` --- the ID of the `vec` object, which value is updated.
- `"value"` --- an array of 2-tuples, where each 2-tuple is a position index
  and an ID (logical timestamp) of the field value.

An example where all timestamps are encoded as 2-tuples:

```json
{
  "op": "ins_vec",
  "obj": [123, 0],
  "value": [
    [0, [123, 1]],
    [1, [123, 2]]
    [3, [123, 3]]
  ]
}
```

Same example, but with timestamps encoded as time differences:

```json
{
  "op": "ins_obj",
  "obj": 1,
  "value": [
    [0, 2],
    [1, 3]
    [2, 4]
  ]
}
```


## `ins_str` Operation Encoding

The `ins_str` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_str"`.
- `"obj"` --- the ID of the `str` object, which value is updated.
- `"after"` --- the ID of the character after which the new sub-string is
  inserted. If the sub-string is inserted at the beginning of the string, then
  this field is set to the ID of the `str` object.
- `"value"` --- a sub-string to insert into the `str` object.

For example:

```json
{ "op": "ins_str", "obj": [123, 0], "after": [123, 1], "value": "foo" }
```


## `ins_bin` Operation Encoding

The `ins_bin` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_bin"`.
- `"obj"` --- the ID of the `bin` object, which value is updated.
- `"after"` --- the ID of the byte after which the new octets are inserted.
  If the sub-string is inserted at the beginning of the `bin` object, then this
  field is set to the ID of the `bin` object.
- `"value"` --- an array of octets to insert into the `bin` object, encoded
  as Base64 string.

For example:

```json
{ "op": "ins_bin", "obj": [123, 0], "after": [123, 1], "value": "Zm9v" }
```

The following alphabet is used for Base64 encoding:

```
ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/
```

The padding character is `=`.


## `ins_arr` Operation Encoding

The `ins_arr` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"ins_arr"`.
- `"obj"` --- the ID of the `arr` object, which value is updated.
- `"after"` --- the ID of the element after which the new elements are inserted.
  If the elements are inserted at the beginning of the `arr` object, then this
  field is set to the ID of the `arr` object.
- `"value"` --- an array of IDs to be inserted into the `arr` object.

For example:

```json
{
  "op": "ins_arr",
  "obj": [123, 0],
  "after": [123, 1],
  "value": [
    [123, 2], [123, 3]
  ]
}
```


## `del` Operation Encoding

The `del` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"del"`.
- `"obj"` --- the ID of the object in which content is deleted.
- `"what"` --- an array for timespans which specifies RGA ranges to be deleted.

For example:

```json
{
  "op": "del",
  "obj": [123, 0],
  "what": [
    [123, 1, 3],
    [123, 10, 1]
  ]
}
```

A *timespan* represents an interval of logical timestamps, and can be encoded
in one of the following ways:

- As a 3-tuple `[sessionId, time, length]`, where
  the `sessionId` and `time` represent the starting point of the timespan,
  and `length` represents the length of the timespan.
- When the `sessionId` is the same as the session ID of the patch, it can be
  omitted. In this case, the timespan is encoded as a
  2-tuple `[timeDiff, length]`. Where the first member `timeDiff` is the
  difference between the starting point of the timespan and the time value
  of the patch ID.


## `nop` Operation Encoding

The `nop` operation is encoded as a JSON object with the following fields:

- `"op"` --- equal to `"nop"`.
- `"len"` --- the number of logical timestamps that are skipped by this
  operation. This field can be omitted, in which case the value is assumed
  to be `1`.

For example:

```json
{ "op": "nop", "len": 10 }
```


---

# JSON CRDT Patch > Encoding > Compact Format

The `compact` encoding follows the [Compact JSON encoding scheme](/specs/compact-json)
which encodes entities as JSON arrays with a special first element that
represents the type of the entity. This results in a very compact
representation of the patch, while still being JSON and human-readable.

Patches encoded using the `compact` format can be serialized to a very compact
binary form using binary JSON encoders, such as CBOR or MessagePack.

A patch consists of a header and a list of operations. The header is a JSON
array with the following elements:

- The first element is the ID of the patch, encoded as a JSON 2-tuple array.
- The second element is an optional metadata object, which can contain any
  additional application specific information. This field can be omitted.

The rest of the elements in the patch are operations, encoded according Compact
JSON encoding scheme.

Below is an example of a patch encoded using the `compact` format:

```json
[
  [
    [123, 456], // Patch ID
    { "author": "John Doe" }, // Optional metadata
  ],
  [1], // "new_obj" operation
  [4]  // "new_str" operation
]
```


## `new_con` Operation Encoding

The `new_con` operation is encoded as a JSON array with the starting
element `0`. The second element is the value of the constant; and the third
element is an optional flag that indicates that the constant is a logical
timestamp.

```json
[0, "foo"]
```

A constant with `undefined` value is encoded by omitting the `"value"` field.

```json
[0]
```

A logical timestamp is encoded by setting the `"timestamp"` flag to `true`.

```json
[0, [123, 456], true]
```

If timestamp session ID is equal to the session ID of the patch, then the
timestamp can be encoded as a single number, which is the logical time
sequence number difference between the timestamp and the ID of the patch.

```json
[0, 10, true]
```


## `new_val` Operation Encoding

The `new_val` operation is encoded as a JSON array with a single element `1`.

```json
[1]
```


## `new_obj` Operation Encoding

The `new_obj` operation is encoded as a JSON array with a single element `2`.

```json
[2]
```


## `new_vec` Operation Encoding

The `new_vec` operation is encoded as a JSON array with a single element `3`.

```json
[3]
```


## `new_str` Operation Encoding

The `new_str` operation is encoded as a JSON array with a single element `4`.

```json
[4]
```


## `new_bin` Operation Encoding

The `new_bin` operation is encoded as a JSON array with a single element `5`.

```json
[5]
```


## `new_arr` Operation Encoding

The `new_arr` operation is encoded as a JSON array with a single element `6`.

```json
[6]
```


## `ins_val` Operation Encoding

The `ins_val` operation is encoded as a JSON array with the starting
element `9`, followed by the ID of the object in which the value is inserted,
and the ID of the new value.

For example:

```json
[9, [123, 0], [123, 1]]
```


## `ins_obj` Operation Encoding

The `ins_obj` operation is encoded as a JSON array with the starting
element `10`, followed by the ID of the object in which the object is inserted,
and an array of 2-tuples, where each 2-tuple is a field name string and an ID
of the new field value.

An example where all timestamps are encoded as 2-tuples:

```json
[10, [123, 0], [
  ["foo", [123, 1]],
  ["bar", [123, 2]]
]]
```

Same example, but with timestamps encoded as time differences:

```json
[10, 1, [
  ["foo", 2],
  ["bar", 3]
]]
```


## `ins_vec` Operation Encoding

The `ins_vec` operation is encoded as a JSON array with the starting
element `11`, followed by the ID of the object in which the vector is inserted,
and an array of 2-tuples, where each 2-tuple is a position index and an ID of
the new field value.

An example where all timestamps are encoded as 2-tuples:

```json
[11, [123, 0], [
  [0, [123, 1]],
  [1, [123, 2]]
  [3, [123, 3]]
]]
```

Same example, but with timestamps encoded as time differences:

```json
[11, 1, [
  [0, 2],
  [1, 3]
  [3, 4]
]]
```


## `ins_str` Operation Encoding

The `ins_str` operation is encoded as a JSON array with the starting
element `12`, followed by the ID of the object in which the string is inserted,
the ID of the character after which the new sub-string is inserted, and the
new sub-string.

For example:

```json
[12, [123, 0], [123, 1], "foo"]
```


## `ins_bin` Operation Encoding

The `ins_bin` operation is encoded as a JSON array with the starting
element `13`, followed by the ID of the object in which the binary object is
inserted, the ID of the byte after which the new octets are inserted, and an
array of octets to insert into the binary object, encoded as Base64 string.

For example:

```json
[13, [123, 0], [123, 1], "Zm9v"]
```

The following alphabet is used for Base64 encoding:

```
ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/
```

The padding character is `=`.


## `ins_arr` Operation Encoding

The `ins_arr` operation is encoded as a JSON array with the starting
element `14`, followed by the ID of the object in which the array is inserted,
the ID of the element after which the new elements are inserted, and an array
of IDs to be inserted into the array.

For example:

```json
[14, [123, 0], [123, 1], [
  [123, 2],
  [123, 3]
]]
```


## `del` Operation Encoding

The `del` operation is encoded as a JSON array with the starting element `16`,
followed by the ID of the object in which content is deleted, and an array of
timespans which specifies RGA ranges to be deleted.

For example:

```json
[16, [123, 0], [
  [123, 1, 3],
  [123, 10, 1]
]]
```

A *timespan* represents an interval of logical timestamps, and can be encoded
in one of the following ways:

- As a 3-tuple `[sessionId, time, length]`, where the `sessionId` and `time` represent
  the starting point of the timespan, and `length` represents the length of the
  timespan.
- When the `sessionId` is the same as the session ID of the patch, it can be
  omitted. In this case, the timespan is encoded as a
  2-tuple `[timeDiff, length]`. Where the first member `timeDiff` is the
  difference between the starting point of the timespan and the time value
  of the patch ID.


## `nop` Operation Encoding

The `nop` operation is encoded as a JSON array with the starting element `17`,
followed by an optional length integer. If the length is omitted, it is assumed
to be `1`.

For example:

```json
[17, 10]
```



---

# JSON CRDT Patch > Encoding > Binary Format

The `binary` JSON CRDT Patch encoding is not human-readable, but is the most
space efficient and performant patch encoding.


## Data Representation

Box notation diagrams are used to represent data structures. Each box represents
an octet (one byte), unless otherwise stated.

```
One byte:
+--------+
|        |
+--------+

Zero or more repeating bytes:
+........+
|        |
+........+

Zero or one byte which ends a repeating byte sequence:
+········+
|        |
+········+

Variable number of bytes:
+========+
|        |
+========+
```

Literal bit contents are shown in boxes in binary format. Or, the name of the
data type may be shown in the boxes.

The following formats are used to encode data:

- Numeric values follow big-endian order: the high order byte precedes the
  lower order byte.
- Strings are encoded using the [UTF-8](https://www.rfc-editor.org/info/rfc3629).
- JSON values are encoded using [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html).


### `u8` Encoding

`u8` stands for *Unsigned 8-bit Integer*, it is encoded as a single byte.

- `z` --- unsigned 8 bit integer

```
+--------+
|zzzzzzzz|
+--------+
```


### `vu57` Encoding

`vu57` stands for *Variable Length Unsigned 57-bit Integer*. It is an unsigned
integer, which is encoded using a variable number---from 1 to 8---of bytes.

- `z` --- variable length unsigned 57 bit integer
- `?` --- whether the next octet is used for encoding

```
byte 1                                                         byte 8
+--------+........+........+........+........+........+........+········+
|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|zzzzzzzz|
+--------+........+........+........+........+........+........+········+

           11111    2211111  2222222  3333332  4443333  4444444 55555555
  7654321  4321098  1098765  8765432  5432109  2109876  9876543 76543210
    |                        |                    |             |
    5th bit of z             |                    |             |
                             28th bit of z        |             57th bit of z
                                                  39th bit of z
```

The `vu57` values are represented in diagrams as a single variable length box:

```
+========+
|  vu57  |
+========+
```


### `b1vu56` Encoding

`b1vu56` stands for *Boolean and Variable Length Unsigned 56-bit Integer*. It
is a single boolean bit flag, followed by a variable length unsigned 56-bit
integer. Each `b1vu56` value is encoded as a variable number---from 1 to 8---of
bytes.

- `f` --- flag
- `z` --- variable length unsigned 56 bit integer
- `?` --- whether the next byte is used for encoding

```
byte 1                                                         byte 8
+--------+........+........+........+........+........+........+········+
|f?zzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|?zzzzzzz|zzzzzzzz|
+--------+........+........+........+........+........+........+········+
 |
 |         1111     2111111  2222222  3333322  4433333  4444444 55555554
 | 654321  3210987  0987654  7654321  4321098  1098765  8765432 65432109
 |  |                        |                    |             |
 |  5th bit of z             |                    |             |
 |                           27th bit of z        |             56th bit of z
 |                                                38th bit of z
 Flag
```

The `b1vu56` values are represented in diagrams as a single variable length box:

```
+========+
| b1vu56 |
+========+
```


### `id` Encoding

The `id` type represents a logical timestamp, which is a 2-tuple
of *session ID* and *time* sequence number. The `id` is encoded as either a
single `b1vu56` value, or as a `b1vu56` value followed by a `vu57` value.

When the session ID of the timestamp being encoded is equal to the session ID
of the patch being encoded, the `id` is encoded as a single `b1vu56` value,
where the bit flag is set to `0`, and the `vu56` value encodes the time
sequence number.

```
 Logical time sequence number (flag = 0)
 |
+========+
| b1vu56 |
+========+
```

In all other cases, the `id` is encoded as a `b1vu56` value followed by
a `vu57` value. The bit flag of the `b1vu56` value is set to `1`, and
the `vu56` value encodes the time sequence number. The `vu57` value encodes
the session ID.

```
 Logical time sequence number (flag = 1)
 |
 |        Session ID
 |        |
+========+========+
| b1vu56 |  vu57  |
+========+========+
```

The `id` values are represented in diagrams as a single variable length box:

```
+========+
|   id   |
+========+
```


## Patch Structure

The patch consists of the following elements:

- The patch ID, encoded as two `vu57` integers. Where first is the session ID,
  and the second is the logical time sequence number. (Patch ID is the ID of
  the first operation in the patch. The IDs of subsequent operations are
  derived by adding the span of a previous operation to the previous operation
  ID.)
- The patch optional metadata. An optional JSON object encoded as CBOR.
  CBOR `undefined` value is used when the metadata is empty.
- The number of operations. Encoded as a `vu57` integer.
- The patch operations. Each operation is encoded as a sequence of bytes one
  after another.

```
  Patch ID
  |                Optional Metadata
  |                |
  |                |        Number of operations
  |                |        |
  |                |        |        Operations
  |                |        |        |
+=================+========+========+=========+
|  vu57  |  vu57  |  CBOR  |  vu57  |   ops   |
+=================+========+========+=========+
```


### Patch ID Encoding

The patch ID is encoded as two `vu57` integers, where the first integer encodes
the session ID, and the second integer encodes the logical time sequence number.

```
 Session ID
 |        Logical time
 |        |
+========+========+
|  vu57  |  vu57  |
+========+========+
```


### Patch Metadata Encoding

A patch may optionally contain an application-specific metadata. The metadata
is encoded as a JSON object using CBOR encoding. If the metadata is empty, it
is encoded as a CBOR `undefined` value.

The CBOR `undefined` value is encoded as a single byte `0xF7`.

```
+--------+
|11110111|
+--------+
```

### Operation Encoding

Operations are encoded as follows:

- First, the total number of operations is encoded as a `vu57` integer.
- Then, each operation is encoded one after another.


#### Operation Header Encoding

Each operation starts with the *operation header*, which is one ore more bytes
long. The operation header encodes the operation opcode, and the length of the
operation payload. When an operation payload does not require a length field,
its bits are simply set to `0`.

When the length is less than or equal to 7, the header is encoded as a single
byte. The first 5 bits of the byte encode
the [the opcode](/specs/json-crdt-patch/overview#Opcodes), and the remaining
3 bits encode the length.

```
 Single header byte
 |
+--------+
|ccccceee|
+--------+
```

Where `ccccc` are the 5 opcode bits, and `eee` are the 3 length bits.

When the length is greater than 7, the header is encoded as two or more bytes.
The first byte only encodes the opcode and all length bits `e` are set to `0`.
The following bytes encode the length as a `vu57` integer.

```
 Opcode byte
 |
 |        Length
 |        |
+--------+========+
|ccccc000|  vu57  |
+--------+========+
```

For operations, where the length value is not used, the length bits are set
to `000` and the header is encoded as a single byte.

```
 Single header byte
 |
+--------+
|ccccc000|
+--------+
```


#### `new_con` Operation Encoding

There are two case for the `new_con` operation:

- When the operation payload is a JSON value or `undefined`.
- When the operation payload is a logical timestamp.

##### When the payload is a JSON value or `undefined`

The `new_con` operation opcode is encoded as `00000` and the length bits are
set to `000`. It is followed by a JSON value or `undefined` encoded as CBOR.

```
+--------+========+
|00000000|  CBOR  |
+--------+========+
```

##### When the payload is a logical timestamp

The `new_con` operation opcode is encoded as `00000` and the length bits are
set to `001`. It is followed by the logical timestamp encoded as `id`.

```
+--------+========+
|00000001|   id   |
+--------+========+
```


#### `new_val` Operation Encoding

The `new_val` operation is encoded as a single byte, where the header opcode
bits are set to `00001` and the length bits are set to `000`.

```
+--------+
|00001000|
+--------+
```


#### `new_obj` Operation Encoding

The `new_obj` operation is encoded as a single byte, where the header opcode
bits are set to `00010` and the length bits are set to `000`.

```
+--------+
|00010000|
+--------+
```


#### `new_vec` Operation Encoding

The `new_vec` operation is encoded as a single byte, where the header opcode
bits are set to `00011` and the length bits are set to `000`.

```
+--------+
|00011000|
+--------+
```


#### `new_str` Operation Encoding

The `new_str` operation is encoded as a single byte, where the header opcode
bits are set to `00100` and the length bits are set to `000`.

```
+--------+
|00100000|
+--------+
```


#### `new_bin` Operation Encoding

The `new_bin` operation is encoded as a single byte, where the header opcode
bits are set to `00101` and the length bits are set to `000`.

```
+--------+
|00101000|
+--------+
```


#### `new_arr` Operation Encoding

The `new_arr` operation is encoded as a single byte, where the header opcode
bits are set to `00110` and the length bits are set to `000`.

```
+--------+
|00110000|
+--------+
```


#### `ins_val` Operation Encoding

The `ins_val` operation opcode is encoded as `00101` and the length bits are
set to `000`. The header byte is followed by two `id` values, which encode the
object being inserted into and the value being inserted, respectively.

```
 Header
 |        The "val" object
 |        |
 |        |        The new value
 |        |        |
+--------+========+========+
|01001000|   id   |   id   |
+--------+========+========+
```


#### `ins_obj` Operation Encoding

The `ins_obj` operation opcode is encoded as `01010` and the length encodes
the number of key-value pairs being inserted. It is followed by the object ID,
and the key-value pairs.

When length is less than or equal to 7, the length is encoded in the first 3
bits of the header byte. The remaining 5 bits are set to `01010`:

```
 Header
 |        The "obj" object
 |        |
 |        |        The new values
 |        |        |
+--------+========+.................+
|01010eee|   id   | key-value pairs |
+--------+========+.................+
```

When length is greater than 7, the length is encoded as a `vu57` integer:

```
 Header
 |        Length (key-value pair count)
 |        |
 |        |        The "obj" object
 |        |        |
 |        |        |        The new values
 |        |        |        |
+--------+========+========+.................+
|01010000|  vu57  |   id   | key-value pairs |
+--------+========+========+.................+
```

Each key-value pair is encoded as a CBOR string followed by an `id`.

```
+========+========+
|  CBOR  |   id   |
+========+========+
```

Consider an example when two key-value pairs are inserted into an object:

```
 Header            Key 1             Key 2
 |        Object   |        Value 1  |        Value 2
 |        |        |        |        |        |
+--------+========+========+========+========+========+
|01010010|   id   |  CBOR  |   id   |  CBOR  |   id   |
+--------+========+========+========+========+========+
```


#### `ins_vec` Operation Encoding

The `ins_vec` operation opcode is encoded as `01011` and the length encodes
the number of elements being inserted. It is followed by the vector ID, and
the element index-value pairs.

When length is less than or equal to 7, the length is encoded in the first 3
bits of the header byte. The remaining 5 bits are set to `01011`:

```
 Header
 |        The "vec" object
 |        |
 |        |        The new values
 |        |        |
+--------+========+...................+
|01011eee|   id   | index-value pairs |
+--------+========+...................+
```

When length is greater than 7, the length is encoded as a `vu57` integer:

```
 Header
 |        Length (key-value pair count)
 |        |
 |        |        The "vec" object
 |        |        |
 |        |        |        The new values
 |        |        |        |
+--------+========+========+...................+
|01011000|  vu57  |   id   | index-value pairs |
+--------+========+========+...................+
```

Each index-value pair is encoded as a `u8` followed by an `id`.

```
+--------+========+
|   u8   |   id   |
+--------+========+
```

Consider an example when two index-value pairs are inserted into a `vec` object:

```
 Header            Index 1           Index 2
 |        Object   |        Value 1  |        Value 2
 |        |        |        |        |        |
+--------+========+--------+========+--------+========+
|01011010|   id   |   u8   |   id   |   u8   |   id   |
+--------+========+--------+========+--------+========+
```


#### `ins_str` Operation Encoding

The `ins_str` operation header opcode is encoded as `01100` and the length
encodes the number bytes the sub-string consumes in UTF-8 format.

The header is followed by the `str` object ID, and the ID of character after
which the sub-string is inserted.

Finally, the last component of the operation is the sub-string itself, encoded
as a sequence of UTF-8 encoded bytes.

When length is less than or equal to 7, the length is encoded in the first 3
bits `eee` of the header byte. The remaining 5 bits are set to `01100`:

```
 Header
 |        The "str" object
 |        |
 |        |        Character after which to insert
 |        |        |
 |        |        |        The new sub-string
 |        |        |        |
+--------+========+========+========+
|01100eee|   id   |   id   |  UTF8  |
+--------+========+========+========+
```

When length is greater than 7, the length is encoded as a `vu57` integer:

```
 Header
 |        Length
 |        |        The "str" object
 |        |        |
 |        |        |        Character after which to insert
 |        |        |        |
 |        |        |        |        The new sub-string
 |        |        |        |        |
+--------+========+========+========+========+
|01100000|  vu57  |   id   |   id   |  UTF8  |
+--------+========+========+========+========+
```


#### `ins_bin` Operation Encoding

The `ins_bin` operation header opcode is encoded as `01101` and the length
encodes the number bytes being inserted.

The header is followed by the ID of the `bin` object being inserted into, and
the ID of the byte after which binary data is inserted.

Finally, the last component of the operation is the binary data itself, encoded
as a sequence of bytes.

When length is less than or equal to 7, the length is encoded in the first 3
bits `eee` of the header byte. The remaining 5 bits are set to `01101`:

```
 Header
 |        The "bin" object
 |        |
 |        |        Octet after which to insert
 |        |        |
 |        |        |        Binary data to insert
 |        |        |        |
+--------+========+========+========+
|01101eee|   id   |   id   |  data  |
+--------+========+========+========+
```

When length is greater than 7, the length is encoded as a `vu57` integer:

```
 Header
 |        Length
 |        |        The "bin" object
 |        |        |
 |        |        |        Octet after which to insert
 |        |        |        |
 |        |        |        |        Binary data to insert
 |        |        |        |        |
+--------+========+========+========+========+
|01101000|  vu57  |   id   |   id   |  data  |
+--------+========+========+========+========+
```


#### `ins_arr` Operation Encoding

The `ins_arr` operation header opcode is encoded as `01110` and the length
encodes the number of elements being inserted.

The header is followed by the ID of the `arr` object being inserted into, and
the ID of the element after which the new elements are inserted.

Finally, the last component of the operation is the elements themselves,
encoded as a sequence of `id`s.

When length is less than or equal to 7, the length is encoded in the first 3
bits of the header byte. The remaining 5 bits are set to `01110`:

When length is less than or equal to 7, the length is encoded in the first 3
bits `eee` of the header byte. The remaining 5 bits are set to `01110`:

```
 Header
 |        The "arr" object
 |        |
 |        |        Element after which to insert
 |        |        |
 |        |        |        New elements to insert
 |        |        |        |
+--------+========+========+=========+
|01110eee|   id   |   id   |   ids   |
+--------+========+========+=========+
```

When length is greater than 7, the length is encoded as a `vu57` integer:

```
 Header
 |        Length
 |        |        The "arr" object
 |        |        |
 |        |        |        Element after which to insert
 |        |        |        |
 |        |        |        |        New elements to insert
 |        |        |        |        |
+--------+========+========+========+=========+
|01110000|  vu57  |   id   |   id   |   ids   |
+--------+========+========+========+=========+
```


#### `del` Operation Encoding

The `del` operation header opcode is encoded as `10000` and the length encodes
the number of timespans being deleted.

The header is followed by the ID of the object being deleted from, and a
sequence of timespans being deleted. Each timespan is encoded as a pair
of `id` and `vu57` integer. The `id` encodes the start of the timespan, and
the `vu57` integer encodes the length of the timespan.

```
 Logical timespan starting timestamp
 |
 |        Length of the timespan
 |        |
+========+========+
|   id   |  vu57  |
+========+========+
```

Below is an example of a `del` operation that deletes 2 timespans:

```
 Header
 |        Object
 |        |        Timespan 1        Timespan 2
 |        |        |                 |
+--------+========+=================+=================+
|10000010|   id   |   id   |  vu57  |   id   |  vu57  |
+--------+========+=================+=================+
```


#### `nop` Operation Encoding

The `nop` operation header opcode is encoded as `10001` and the length encodes
the number logical clock cycles it consumes.

For example, below encoding shows a `nop` operation that consumes 2 logical
clock cycles:

```
+--------+
|10001010|
+--------+
```


---

# JSON CRDT Patch > Encoding > Examples

Lets consider a scenario where we want to set the document root to a new object,
which has a single key `"foo"`, which is set to a string `"bar"`.

```json
{
  "foo": "bar"
}
```

We will use `123` as the session ID and `456` as the sequence number of the
first operation. We will need to:

1. Create a new string using the `new_str` operation.
2. Insert the text `"bar"` into the string using the `ins_str` operation.
3. Create a new object using the `new_obj` operation.
4. Insert the key `"foo"` into the object using the `ins_obj` operation.
5. Set the document root value to the new object using the `ins_val` operation.

The patch could look like this in human-readable form:

```
Patch 123.456!7
├─ new_str 123.456
├─ ins_str 123.457!3, obj = 123.456 { 123.456 ← "bar" }
├─ new_obj 123.460
├─ ins_obj 123.461!1, obj = 123.460
│   └─ "foo": 123.456
└─ ins_val 123.462!1, obj = 0.0, val = 123.460
```

In `verbose` encoding, the patch would look like below and would consume 231
bytes, when encoded as JSON:

```json
{
  "id": [123, 456],
  "ops": [
    { "op": "new_str" },
    {
      "op": "ins_str",
      "obj": [123, 456],
      "after": [123, 456],
      "value": "bar"
    },
    { "op": "new_obj" },
    {
      "op": "ins_obj",
      "obj": [123, 460],
      "value": [
        [ "foo", [123, 456] ]
      ]
    },
    {
      "op": "ins_val",
      "obj": [0, 0],
      "value": [123, 460]
    }
  ]
}
```

In `compact` encoding, the patch would consume just 77 bytes if encoded as JSON
and mere 46 bytes if encoded as CBOR, it would be formatted as below:

```json
[
  [
    [123, 456]
  ],
  [4],
  [12, 456, 456, "bar"],
  [2],
  [10, 460, [
    ["foo", 456]
  ]],
  [9, [0, 0], 460]
]
```

In `binary` encoding, the patch would consume just 29 bytes, encoded in hex as
follows:

```
7b c8 03 f7 05 04 6c c8
07 c8 07 62 61 72 02 2a
cc 07 63 66 6f 6f c8 07
09 00 00 cc 07
```

---

# JSON CRDT Patch > License

json-joy JSON CRDT Patch specification is licensed under the
[__Attribution-ShareAlike 4.0 International__ (CC BY-SA 4.0)][license] license.

[![](https://i.creativecommons.org/l/by-sa/4.0/88x31.png)][license]

[license]: https://creativecommons.org/licenses/by-sa/4.0/


