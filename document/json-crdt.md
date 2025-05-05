---

# JSON CRDT

The JSON CRDT specification describes how to implement a full JSON-like
conflict-free replicated data type (CRDT). JSON CRDT specification works hand
in hand with the [JSON CRDT Patch][json-crdt-patch] specification, which
describes how changes to JSON CRDT documents are represented.

[json-crdt-patch]: /specs/json-crdt-patch


---

# JSON CRDT > Model Document

The Model Document is the highest level of abstraction in the JSON CRDT
specification. It contains all the data and metadata necessary to construct and
operate a single JSON-like document, which has all the metadata necessary to
apply JSON CRDT Patch operations to it.

~~~jj.note
Although this document mostly mentions JSON data structures, the same
specification can be extended to CBOR data structures as well. When this
document uses the term JSON it refers to both JSON and CBOR.

JSON CRDT supports all JSON data types,
including *null*, *boolean*, *number*, *string*, *array*, and *object*; but, in
addition, it also supports *binary* and *undefined* data types, which are not
part of the JSON specification, but are supported by CBOR.
~~~


---

# JSON CRDT > Model Document > Model

The *model* of a JSON CRDT document, `model`, is a collection of the underlying
data, metadata, auxiliary abstract data structures, and algorithms which are
necessary for every JSON CRDT document. Each JSON CRDT document is composed of
many small CRDT nodes, the task of the model is to define semantics of the CRDT
nodes and how they interact with each other. The CRDT nodes are connected
together in a tree-like structure, which forms a JSON-like document. This
specification defines the semantics of the model---all the CRDT algorithms
and their interactions---but it does not define the concrete data structures
used to implement the model.

The *view* of a JSON CRDT document, `model.view`, is a JSON or CBOR data value,
which is derived from the `model`, it can be serialized and deserialized to and
from a JSON or CBOR data format. The view is exposed to the application and is
guaranteed to be consistent across all replicas of the JSON CRDT document. In
other words the view is the JSON value that the application sees and interacts
with. The application can assign any meaning to the view.

JSON CRDT model uses the same [logical clock][logical-clock], `model.clock`, as
the JSON CRDT Patch specification. The logical clock is used to assign unique
identifiers---logical timestamps---to all CRDT nodes and other metadata.

Each JSON CRDT document is a collection of CRDT nodes. The nodes are organized
in two ways: structural and indexed. The structural organization of the nodes
is defined by the tree-like structure of the JSON CRDT document, due to node
ability to reference other nodes. The root node of the tree is
the `model.root` node. The indexed organization of the nodes is defined by
the `model.index` data structure, which maps logical timestamps to CRDT nodes.
The `model.index` is used to quickly lookup CRDT nodes by their ID (logical
timestamp).

Finally, all changes to the JSON CRDT model are made by applying
[JSON CRDT Patch][json-crdt-patch] operations. The `model.apply()` procedure
applies a JSON CRDT Patch changes to the model.


[json-crdt-patch]: /specs/json-crdt-patch
[logical-clock]: /specs/json-crdt-patch/patch-document/logical-clock


---

# JSON CRDT > Model Document > CRDT Algorithms

JSON CRDT specifies just three CRDT algorithms out of which all JSON CRDT
documents constructed. Every JSON CRDT node implements one of the three
algorithms. The algorithms are: (1) constant; (2) last-write-wins
register; and (3) replicated growable array.

The constant algorithm is really just a degenerate case of the most basic CRDT
algorithm---one that does not allow any concurrent edits---hence really only two
actual CRDT algorithms are specified: last-write-wins register and replicated
growable array.


## Constant CRDT Algorithm

The *Constant CRDT* is a special case of a CRDT, which allows to create an
atomic immutable constant value. The constant value is set once at the creation
of the CRDT node and cannot be changed afterwards.

~~~jj.note
Only the `con` Constant JSON CRDT node type uses the Constant CRDT algorithm.
~~~


## Last-Write-Wins (LWW) CRDT Algorithm

The *Last-Write-Wins* (LWW) algorithm is a CRDT algorithm, which allows to
create a mutable register, which stores a single value, which can be
concurrently edited by multiple clients. The value of the register is a logical
timestamp, which is an ID that links to another JSON CRDT node.

~~~jj.aside
The LWW CRDTs do not use the operation ID to determine the order
of concurrent edits, instead they use the logical clock of the value.
This reduces the amount of metadata that needs to be stored in the CRDT
document. It also prevents circular relationships between CRDT nodes.
~~~

The LWW CRDTs use the logical clock of the value to determine which value wins
in case of concurrent edits. The value with the highest logical clock wins.

~~~jj.note
See next, `val`, `obj`, and `vec` are the three JSON CRDT node types which use
the LWW algorithm. The `val` represents a single JSON/CBOR value, the
`obj` is a map of key-value pairs, and the `vec` is vector (array without gaps)
of values.
~~~

The LWW CRDTs support only an insertion operation, specifically the `ins_val`,
`ins_vec`, and `ins_obj` JSON CRDT Patch operations insert new values into the
LWW CRDT nodes.

~~~jj.aside
This also implies that JSON CRDT does not support "move" operation between two
different nodes.
~~~

The ID of the inserted value must also be greater than the ID of the JSON CRDT
node into which the value is inserted. Otherwise, the insertion fails. This
ensures that there are no circular references between CRDT nodes.

### LWW Insertion Routine

Insertion follows the simple routine where the highest logical clock wins:

1. Compare the ID of the new value with the the ID of the container node. If the
   ID of the container node is less than the ID of the new value, proceed to
   step 2. Otherwise, the insertion fails.
2. If the register is empty, proceed to step 4. Otherwise, proceed to step 3.
3. Compare the ID of the new value with the logical clock of the current value.
   If the ID of the new value is greater than the logical clock of the current
   value, proceed to step 4. Otherwise, the insertion fails.
4. Replace the current value with the new value.


## Replicated Growable Array (RGA) CRDT Algorithm

The *Replicated Growable Array* (RGA) algorithm is used for all JSON CRDT nodes
which implement an ordered lists of values. The RGA algorithm allows users to
concurrently insert and remove elements from the list and then merge the
lists together.

~~~jj.note
See next, `str`, `bin`, and `arr` JSON CRDT node types use the RGA algorithm.
The `str` node represents a string, the `bin` node is a binary blob, and
the `arr` node is an array.
~~~

Each RGA sorted list is composed of *elements*. An element has different type
depending on the node type. Each element in the RGA sorted list is
identified by a unique identifier, which is represented as a logical timestamp.

RGA nodes in JSON CRDT specification support *block-wise* internal
representation, where consecutive elements are stored in a
single *block* (or *chunk*), if the logical timestamps of consecutive elements
are consecutive, i.e. the the session IDs are the same and the sequence numbers
are consecutively incrementing. An implementation MAY choose to not use
the block-wise internal representation, in which case each element is stored in
a separate block.

~~~jj.note
This specification uses the RGA algorithm due to its popularity and efficiency.
The RGA algorithm is the most cited list CRDT algorithm in the literature and
the most often used in practice.

It is also very efficient with regards to the amount of metadata that needs to
be stored in the CRDT document. When serialized JSON CRDT documents store a
single logical timestamp per block (collection of consecutive elements) of
metadata overhead. Each logical timestamps consumes on average 2-3 bytes of
storage space.

This specification does not prescribe any particular RGA implementation, but is
possible to implement the RGA algorithm such that all local and remote operations
take no more than logarithmic time.
~~~

CRDT nodes, which use the RGA algorithm, support: (1) insertion `ins_*`;
and (2) deletion `del` operations. The insertion operations insert new elements
after some existing element in the RGA sorted list. The deletion operation
marks some existing elements as deleted.

### RGA Insertion Routine

One or more elements can be inserted into the RGA sorted list nodes by
the [`ins_str`][ins_str], [`ins_bin`][ins_bin], and [`ins_arr`][ins_arr]
operations.

[ins_str]: http://localhost:8081/specs/json-crdt-patch/patch-document/operations#The-ins_str-Operation
[ins_bin]: http://localhost:8081/specs/json-crdt-patch/patch-document/operations#The-ins_bin-Operation
[ins_arr]: http://localhost:8081/specs/json-crdt-patch/patch-document/operations#The-ins_arr-Operation

~~~jj.aside
When a new list of elements is stored as a single block, then only the ID of
the first element in the list is stored in the block.
~~~

When a new list of elements is inserted into the RGA node each element in the
list is assigned a unique ID. The ID of the first element in the list is the ID
of the operation that inserted the list. The IDs of the subsequent elements are
consecutive logical timestamps, i.e. the session IDs are the same and the
sequence numbers are consecutively incremented.

Also, each insert operation specifies the ID of the element after which the new
elements are inserted. If the new elements shall be inserted at the beginning of
the list, then the ID, which specifies after which the new elements are inserted
is set to the ID of the RGA node.

New elements are inserted into the sorted list following the RGA insertion
`rga.insert()` procedure. The `rga.insert()` procedure defines the following
parameters:

- `rga.node` --- ID of the RGA node.
- `rga.ref` --- ID of the element after which to insert.
- `rga.elems` --- List of elements to insert.

The `rga.insert()` algorithm is defined as follows, for
each `elem` in `rga.elems`:

1. Insertion cursor is set to the position before all elements in the RGA node.
2. If the `rga.ref` is not equal to the `rga.node`, then the cursor is moved to
   the position right after the element with the `rga.ref` ID.
3. If the ID of the element after the cursor is greater than the ID of `elem`,
   then the cursor is moved one position forward and step 3 is repeated.
   Otherwise, continue to step 4.
4. If the ID of the element after the cursor is equal to the ID of `elem`,
   then the insertion stops. The elements have already been inserted by
   a previous application of this algorithm. Otherwise, continue to step 5.
5. Insert the `elem` at the cursor position.

### RGA Deletion Routine

Deletion of elements is done by the [`del` operation][del]. The `del` operation
specifies the ID of the RGA node from which the elements are deleted, as well as
a list of IDs of the elements to delete.

[del]: /specs/json-crdt-patch/patch-document/operations#The-del-Operation

The deletion marks all the specified elements as deleted, i.e. creates
tombstones. The deleted elements are not removed from the RGA list, but are
instead marked as deleted. However, the contents of the deleted elements can be
discarded.

When the view of the RGA node is generated, the deleted elements are omitted
from the view.


---

# JSON CRDT > Model Document > Node Types

Nodes are building blocks, which compose a JSON CRDT document. Each node is
itself a CRDT, which is powered by one of the three CRDT algorithms specified
above. The JSON CRDT specification defines seven different node types: (1)
`con`, a constant; (2) `val`, a LWW-Value; (3) `obj`, a LWW-Object; (4) `vec`, a
LWW-Vector; (5) `str`, an RGA-String; (6) `bin`, an RGA-Binary; and (7) `arr`, a
RGA-Array.

Each node is identified by a unique identifier (ID), which is represented as a
logical timestamp. Additionally, each node has a type, which specifies the CRDT
algorithm that node implements

JSON CRDT specifies three categories of data types: (1) a constant; (2) LWW
(Last-Write-Wins) objects; (3) RGA (Replicated Growable Array) objects.

The constant data type represents a value that is never changed. The value of a
constant data type is set at its creation and cannot be changed afterwards.

The LWW data types are objects that can be modified by overwriting a whole field
by a new value, if the new value is newer than the current value. JSON CRDT
defines three LWW data types: (1) `val` a LWW-Value; (2) `obj` a LWW-Object;
and (3) `vec` a LWW-Vector.

The RGA data types are ordered lists of values. All ordered lists in JSON CRDT
are powered by the RGA (Replicated Growable Array) algorithm. JSON CRDT defines
three RGA data types: (1) `str` an RGA-String; (2) `bin` an RGA-Binary; and (3)
`arr` a RGA-Array.

The nodes can be grouped into two categories: (1) nodes that store raw values,
(2) nodes that store references to other nodes. The nodes that store raw values
are: `con`, `str`, and `bin`. The nodes that only store references to other
nodes are: `val`, `obj`, `vec`, and `arr`.


### The `con` --- Constant Node Type

The Constant `con` node represents an immutable atomic value, which is set at
the creation of the node and cannot be changed afterwards.

Like all JSON CRDT nodes, the `con` node has a unique identifier, which is
represented as a logical clock.

The value of the `con` node can be one of the following:

- any JSON/CBOR value, including container types, such as `object` and `array`,
  and binary data;
- `undefined` literal;
- A logical timestamp.

The `con` node can be used to create a constant value, which can be referenced
by other nodes. For example, the `arr` array node elements can reference a
constant value, which is defined by a `con` node. Or any of the LWW nodes can
reference a constant value, which is defined by a `con` node.

~~~jj.note
For example, a simple JSON CRDT document with value set to `42` can be
represented by the root LWW register which contains a reference to the `con`
node, which in turn contains the value `42`.

    val
    └─ con { 42 }
~~~

Each `con` node is composed of the following parts:

- `con.id` --- node ID, as a logical timestamp.
- `con.value` --- the raw data, which can be any JSON/CBOR value.

The view of the `con` node is the value of the `con.value` field.


### The `val` --- LWW-Value Node Type

The LWW-Value `val` node represents a single mutable last-write-wins value,
which can be concurrently edited by multiple clients.

A new `val` node is created by the `new_val` operation. Each `val` node is
identified by its unique ID (a logical timestamp), which is assigned to be the
ID of the `new_val` operation that created the `val` node.

Each `val` node has a value, which is an ID (a logical timestamp) of another
node. At creation time the `new_val` operation assigns the initial value to the
`val` node.

The value of the `val` can be changed by the `ins_val` operation. The `ins_val`
operation contains the ID of the new value, which is assigned to the `val` node.
The operation is successful only if the new value has a higher logical clock
than the current value of the `val` node.

Each `val` node is composed of the following parts:

- `val.id` --- node ID, as a logical timestamp.
- `val.value` --- the value, as a logical timestamp. It is a reference to
  another CRDT node.

The view of the `val` node is the view of the node referenced by the `val.value`
field.


### The `obj` --- LWW-Object Node Type

The LWW-Object `obj` node represents a mutable map of key-value pairs, which can
be concurrently edited by multiple clients. Each key in the map is a string.
Each key in the map is associated with a value, which is an last-write-wins
register.

Each `obj` node is created by the `new_obj` JSON CRDT Patch operation. Each
`obj` node is identified by its unique ID (a logical timestamp), which is
assigned to be the ID of the `new_obj` operation that created the `obj` node.

The `obj` node contains a map of key-value pairs. Each key in the map is a
string. Each value in the map is an ID (a logical timestamp) of another node.
Initially, the map is empty.

The `obj` node can be modified by the `ins_obj` operation. The `ins_obj`
operation contains a list of key-value pairs to insert into the map. Each
key-value pair insertion is successful only if the the ID of the value node is
higher than the ID of the current value of that key, or if the key does not
exist in the map. On success, the old value of the key is replaced by the new
value.

To delete a key-value pair from the map, also the `ins_obj` operation is used.
In this case the value node ID is set to point to a `con` node with a raw value
of `undefined`. When the view of the `obj` node is generated, the key-value
pairs with the value set to `undefined` are omitted from the map.

Each `obj` node is composed of the following parts:

- `obj.id` --- node ID, as a logical timestamp.
- `obj.map` --- a map of key-value pairs, where each key is a string and each
  value is an ID (a logical timestamp) of another node.

The view of the `obj` node is a JSON object, which contains the key-value pairs
from the `obj.map` field. The keys are the keys from the `obj.map` field. The
values are the views of the nodes referenced by the `obj.map` field.


### The `vec` --- LWW-Vector Node Type

The LWW-Vector `vec` node represents a mutable vector---array without gaps. The
vector might grow in size, but it cannot shrink. A typical use case is for
fixed size tuples.

Each `vec` node is created by the `new_vec` JSON CRDT Patch operation. Each
`vec` node is identified by its unique ID (a logical timestamp), which is
assigned to be the ID of the `new_vec` operation that created the `vec` node.

The `vec` node stores an index-value mapping for all elements in the vector.
Each index is a non-negative integer. The maximum number of elements in a vector
is 256, hence the minimum index is 0 and the maximum index is 255. Each value in
the map is an ID (a logical timestamp) of another node.

Initially, the vector is empty. The `vec` node can be modified by the `ins_vec`
operation. The `ins_vec` operation contains a list of index-value pairs to
insert into the vector. Each index-value pair insertion is successful only if
the the ID of the value node is higher than the ID of the current value at that
index, or if the index does not exist in the vector. On success, the old value
at the index is replaced by the new value.

If an index-value pair is inserted at an index, which is greater than the
current size of the vector, then the vector is grown to the new size. If that
index is greater than the maximum size of the vector, then the operation fails.
If vector growth results in a gap, then the gap is filled with `undefined` when
the view of the vector is generated.

Each `vec` node is composed of the following parts:

- `vec.id` --- node ID, as a logical timestamp.
- `vec.map` --- a map of index-value pairs, where each index is a non-negative
  integer and each value is an ID (a logical timestamp) of another node.

The view of the `vec` node is a JSON array, which contains the values from the
`vec.map` field. The values are the views of the nodes referenced by the
`vec.map` field. The gaps in the array are filled with `undefined`.


### The `str` --- RGA-String Node Type

The RGA-String `str` node represents a mutable string (ordered list of text
elements), which can be concurrently edited by multiple clients. The `str` nodes
are powered by the RGA (Replicated Growable Array) algorithm. Each *element* in
the RGA-String is UTF-16 code unit.

Every `str` node is created by the `new_str` operation. Each `str` node is
identified by its unique ID (a logical timestamp), which is assigned to be the
ID of the `new_str` operation that created the `str` node.

Modifications to the `str` node are made by the `ins_str` and `del` operations.
The `ins_str` operation inserts one or more consecutive elements into the
string. The `del` operation deletes one or more elements from the string.

Each `str` node is composed of the following parts:

- `str.id` --- node ID, as a logical timestamp.
- `str.chunks` --- a sorted list of blocks (chunks) of consecutive elements,
  where each block contains:
    - `str.chunks[n].id` --- a block ID (a logical timestamp), which is the ID
      of the first element in the block. The IDs of the other consecutive
      elements in the block are derived by incrementing the sequence number of
      the block ID.
    - `str.chunks[n].data` --- a list of elements, which are UTF-16 code units,
      if the block is not deleted.
    - `str.chunks[n].isDeleted` --- a flag, which indicates whether the block is
      deleted or not.

The view of the `str` node is a string, which is generated by concatenating the
elements from the `str.chunks` field.


### The `bin` --- RGA-Binary Node Type

The RGA-Binary `bin` node represents a sorted list of mutable binary data, which
can be concurrently edited by multiple clients. The `bin` nodes are powered by
the RGA (Replicated Growable Array) algorithm. Each element in the `bin` node is
a single octet (8-bit byte).

Every `bin` node is created by the `new_bin` operation. Each `bin` node is
identified by its unique ID (a logical timestamp), which is assigned to be the
ID of the `new_bin` operation that created the `bin` node.

Modifications to the `bin` node are made by the `ins_bin` and `del` operations.
The `ins_bin` operation inserts one or more consecutive elements into the
binary. The `del` operation deletes one or more elements from the binary.

Each `bin` node is composed of the following parts:

- `bin.id` --- node ID, as a logical timestamp.
- `bin.chunks` --- a sorted list of blocks (chunks) of consecutive elements,
  where each block contains:
    - `bin.chunks[n].id` --- a block ID (a logical timestamp), which is the ID
      of the first element in the block. The IDs of the other consecutive
      elements in the block are derived by incrementing the sequence number of
      the block ID.
    - `bin.chunks[n].data` --- a list of elements, which are octets (8-bit
      bytes), if the block is not deleted.
    - `bin.chunks[n].isDeleted` --- a flag, which indicates whether the block is
      deleted or not.

The view of the `bin` node is a binary blob, which is generated by concatenating
the elements from the `bin.chunks` field.


### The `arr` --- RGA-Array Node Type

The RGA-Array `arr` node represents a mutable ordered list of values, which can
be concurrently edited by multiple clients. The `arr` nodes are powered by the
RGA (Replicated Growable Array) algorithm. Each element in the `arr` node is a
logical timestamp, which is the ID of another CRDT node.

Every `arr` node is created by the `new_arr` operation. Each `arr` node is
identified by its unique ID (a logical timestamp), which is assigned to be the
ID of the `new_arr` operation that created the `arr` node.

Modifications to the `arr` node are made by the `ins_arr` and `del` operations.
The `ins_arr` operation inserts one or more consecutive elements into the
array. The `del` operation deletes one or more elements from the array.

Each `arr` node is composed of the following parts:

- `arr.id` --- node ID, as a logical timestamp.
- `arr.chunks` --- a sorted list of blocks (chunks) of consecutive elements,
  where each block contains:
    - `arr.chunks[n].id` --- a block ID (a logical timestamp), which is the ID
      of the first element in the block. The IDs of the other consecutive
      elements in the block are derived by incrementing the sequence number of
      the block ID.
    - `arr.chunks[n].data` --- a list of elements, which are logical timestamps,
      if the block is not deleted.
    - `arr.chunks[n].isDeleted` --- a flag, which indicates whether the block is
      deleted or not.

The view of the `arr` node is a JSON array, which contains the values from the
`arr.chunks` field. The values are the views of the nodes referenced by the
`arr.chunks` field.


---

# JSON CRDT > Model Document > Node Composition

JSON CRDT document is a collection of CRDT nodes. The nodes are organized such
that they form a tree. The tree is rooted at the root node, which is a `val`
(LWW-Value) node with ID `0.0`. The nodes form a graph, because nodes can
reference other nodes. This is called the *structure* of the JSON CRDT document.

The structure of a JSON CRDT document starts with the root node---`model.root`,
which is a `val` (LWW-Value) node with ID `0.0`. The root node is the entry
point to the JSON CRDT document.

Any subsequent `ins_val` operation on the root node will change its
value as it will have a higher logical clock than the initial value of `0.0`.

JSON CRDT nodes fall into two natural categories: (1) nodes that store raw
data, (2) nodes that store references to other nodes. The nodes that store raw
data are: `con`, `str`, and `bin`. The nodes that only store references to
other nodes are: `val`, `obj`, `vec`, and `arr`.

The model *index*, `model.index`, is a data structure that maps logical
timestamps to CRDT nodes. It allows for fast lookup of CRDT nodes by their
logical timestamp.

JSON CRDT nodes are not allowed to form cycles, i.e. a node cannot reference
itself or any of its ancestors. This is prevented by the way how JSON CRDT
algorithms are defined.

~~~jj.note
##### Empty Document Example

Every JSON CRDT document has a root node, which is the `0.0` LWW-Value node.
When a new document is created the value of the root node is set to `0.0`, it
points to the `0.0` Constant node, which contains the raw value of `undefined`.

```
model.root
└─ val 0.0
   └─ con 0.0 { undefined }
```

The node index in an empty document is empty, as the `0.0` Constant node is
virtual and does not exist in the index. The `model.view` is `undefined` and
the `model.clock` will assign the first sequence number `1` to the first JSON
CRDT Patch operation.

```
model
├─ root
│  └─ val 0.0
│     └─ con 0.0 { undefined }
│
├─ index
│
├─ view
│  └─ undefined
│
└─ clock x.1
```
~~~

~~~jj.note
##### JSON Object Example

Consider we want to model the following JSON object as a CRDT:

```js
{
   "foo": "bar",
   "baz": {
      "qux": 123,
      "quux": [1, 2, 3]
   }
}
```

There are a number of ways to model this object as a CRDT. One way would be
using the following nodes:


```
model.root
└─ val 0.0
   └─ obj x.1
      ├─ "foo"
      │   └─ str x.2 { "bar" }
      └─ "baz"
          └─ obj x.3
             ├─ "qux"
             │   └─ con x.4 { 123 }
             └─ "quux"
                 └─ arr x.5
                    ├─ [0]: val x.6
                    │       └─ con x.7 { 1 }
                    ├─ [1]: val x.8
                    │       └─ con x.9 { 2 }
                    └─ [2]: val x.10
                            └─ con x.11 { 3 }
```

Another way could be to express a JSON array as a `vec` node instead of an
`arr` node, and not wrap the array elements in `val` nodes, this would
result in a more space efficient model. Also, let's make the `"bar"` string
a `con` instead of a `str` node. It would make the `"bar"` string immutable,
but save space. The resulting structure would be:

```
model.root
└─ val 0.0
   └─ obj x.1
      ├─ "foo"
      │   └─ con x.2 { "bar" }
      └─ "baz"
          └─ obj x.3
             ├─ "qux"
             │   └─ con x.4 { 123 }
             └─ "quux"
                 └─ vec x.5
                    ├─ [0]: con x.6 { 1 }
                    ├─ [1]: con x.7 { 2 }
                    └─ [2]: con x.8 { 3 }
```

The full `model` structure could be expressed as follows:

```
model
├─ root
│  └─ val 0.0
│     └─ obj x.1
│        ├─ "foo"
│        │   └─ con x.2 { "bar" }
│        └─ "baz"
│            └─ obj x.3
│               ├─ "qux"
│               │   └─ con x.4 { 123 }
│               └─ "quux"
│                   └─ vec x.5
│                      ├─ [0]: con x.6 { 1 }
│                      ├─ [1]: con x.7 { 2 }
│                      └─ [2]: con x.8 { 3 }
│
├─ index
│  ├─ x.1: obj
│  ├─ x.2: con { "bar" }
│  ├─ x.3: obj
│  ├─ x.4: con { 123 }
│  ├─ x.5: vec
│  ├─ x.6: con { 1 }
│  ├─ x.7: con { 2 }
│  └─ x.8: con { 3 }
│
├─ view
│  └─ {
│        "foo": "bar",
│        "baz": {
│           "qux": 123,
│           "quux": [1, 2, 3]
│        }
│     }
│
└─ clock x.9
```
~~~

JSON CRDT Patch operations can be applied to a JSON CRDT document to modify its
internal model and view. The next section describes the JSON CRDT Patch
operation semantics.


---

# JSON CRDT > Model Document > Operation Semantics

All changes to a JSON CRDT document happen by *applying* [JSON CRDT Patch
operations][operations]. The application is done by the `model.apply()` method,
which semantics is described in the following section.

[operations]: /specs/json-crdt-patch/patch-document/operations

The `model.apply()` method is idempotent, it can be called multiple times with
the same JSON CRDT Patch document and it will always produce the same result.


## The `new_con` Operation

The [`new_con` operation][new-con-op] is applied to a JSON CRDT document by the
following routine:

[new-con-op]: /specs/json-crdt-patch/patch-document/operations#The-new_con-Operation

1. The `model.index` is checked to see if a node with `new_con.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `con` node is created.
3. The new `con` node inherits the ID of the `new_con` operation: `con.id` is
   set to `new_con.id`.
4. The raw data `con.value` is set to `new_con.value`.
5. The newly created `con` node is added to the `model.index`.


## The `new_val` Operation

The [`new_val` operation][new-val-op] is applied to a JSON CRDT document by the
following routine:

[new-val-op]: /specs/json-crdt-patch/patch-document/operations#The-new_val-Operation

1. The `model.index` is checked to see if a node with `new_val.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `val` node is created.
3. The new `val` node inherits the ID of the `new_val` operation: `val.id` is
   set to `new_val.id`.
4. The value of the `val` node `val.value` is set to `new_val.value`.
5. The newly created `val` node is added to the `model.index`.


## The `new_obj` Operation

The [`new_obj` operation][new-obj-op] is applied to a JSON CRDT document by the
following routine:

[new-obj-op]: /specs/json-crdt-patch/patch-document/operations#The-new_obj-Operation

1. The `model.index` is checked to see if a node with `new_obj.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `obj` node is created.
3. The new `obj` node inherits the ID of the `new_obj` operation: `obj.id` is
   set to `new_obj.id`.
4. The newly created `obj` node is added to the `model.index`.


## The `new_vec` Operation

The [`new_vec` operation][new-vec-op] is applied to a JSON CRDT document by the
following routine:

[new-vec-op]: /specs/json-crdt-patch/patch-document/operations#The-new_vec-Operation

1. The `model.index` is checked to see if a node with `new_vec.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `vec` node is created.
3. The new `vec` node inherits the ID of the `new_vec` operation: `vec.id` is
   set to `new_vec.id`.
4. The newly created `vec` node is added to the `model.index`.


## The `new_str` Operation

The [`new_str` operation][new-str-op] is applied to a JSON CRDT document by the
following routine:

[new-str-op]: /specs/json-crdt-patch/patch-document/operations#The-new_str-Operation

1. The `model.index` is checked to see if a node with `new_str.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `str` node is created.
3. The new `str` node inherits the ID of the `new_str` operation: `str.id` is
   set to `new_str.id`.
4. The newly created `str` node is added to the `model.index`.


## The `new_bin` Operation

The [`new_bin` operation][new-bin-op] is applied to a JSON CRDT document by the
following routine:

[new-bin-op]: /specs/json-crdt-patch/patch-document/operations#The-new_bin-Operation

1. The `model.index` is checked to see if a node with `new_bin.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `bin` node is created.
3. The new `bin` node inherits the ID of the `new_bin` operation: `bin.id` is
   set to `new_bin.id`.
4. The newly created `bin` node is added to the `model.index`.


## The `new_arr` Operation

The [`new_arr` operation][new-arr-op] is applied to a JSON CRDT document by the
following routine:

[new-arr-op]: /specs/json-crdt-patch/patch-document/operations#The-new_arr-Operation

1. The `model.index` is checked to see if a node with `new_arr.id` already
   exists. If it does, the operation is ignored and the application routine
   terminates. Otherwise, the application routine continues.
2. A new `arr` node is created.
3. The new `arr` node inherits the ID of the `new_arr` operation: `arr.id` is
   set to `new_arr.id`.
4. The newly created `arr` node is added to the `model.index`.


## The `ins_val` Operation

The `ins_val` operation changes the value of an existing `val` node following
the LWW algorithm semantics. The [`ins_val` operation][ins-val-op] is applied
to a JSON CRDT document by the following routine:

[ins-val-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_val-Operation

1. A `val` node with ID `ins_val.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of a `val` node type, the
   operation is ignored and the application routine terminates.
2. The value `val.value` of the `val` node is changed to `ins_val.value` if and
   only if all the following conditions are met:
   1. The new value `ins_val.value` is greater than the current value `val.value`.
   2. And the new value `ins_val.value` is greater than the node ID `val.id`.


## The `ins_obj` Operation

The `ins_obj` operation changes values of an existing `obj` node following the
LWW algorithm semantics for each key-value pair in the operation.
The [`ins_obj` operation][ins-obj-op] is applied to a JSON CRDT document by the
following routine:

[ins-obj-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_obj-Operation

1. An `obj` node with ID `ins_obj.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of an `obj` node type, the
   operation is ignored and the application routine terminates.
2. For each key `k` in `ins_obj.map` key-value map:
   1. If the value `ins_obj.map[k]` is less or equal to the node ID `obj.id`,
      the key-value pair is ignored. Otherwise, the application routine
      continues.
   2. If the key `k` does not exist in `obj.map`, the key-value pair is
      added to the `obj.map`.
   3. If the key `k` already exists in `obj.map`, the value `obj.map[k]` is
      changed to `ins_obj.map[k]` if and only if the new value `ins_obj.map[k]`
      is greater than the current value `obj.map[k]`.


## The `ins_vec` Operation

The `ins_vec` operation changes values of an existing `vec` node following the
LWW algorithm semantics for each index-value pair in the operation.
The [`ins_vec` operation][ins-vec-op] is applied to a JSON CRDT document by the
following routine:

[ins-vec-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_vec-Operation

1. An `vec` node with ID `ins_vec.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of a `vec` node type, the
   operation is ignored and the application routine terminates.
2. For each index `i` in `ins_vec.map` index-value map:
   1. If the index `i` is less than 0 or greater than 255, the index-value pair
      is ignored and the application routine terminates. Otherwise, the
      application routine continues.
   2. If the value `ins_vec.map[i]` is less or equal to the node ID `vec.id`,
      the index-value pair is ignored. Otherwise, the application routine
      continues.
   3. If the index `i` does not exist in `vec.map`, the index-value pair is
      added to the `vec.map`.
   4. If the index `i` already exists in `vec.map`, the value `vec.map[k]` is
      changed to `ins_vec.map[k]` if and only if the new value `ins_vec.map[k]`
      is greater than the current value `vec.map[k]`.


## The `ins_str` Operation

The `ins_str` operation inerts a sub-string into an existing `str` node
following the RGA algorithm semantics. The [`ins_str` operation][ins-str-op]
is applied to a JSON CRDT document by the following routine:

[ins-str-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_str-Operation

1. A `str` node with ID `ins_str.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of a `str` node type, the
   operation is ignored and the application routine terminates.
2. The sub-string `ins_str.data` is inserted into the `str` node by the
   `rga.insert()` procedure, where the parameters are:
   - `rga.node` is set to `str.id`.
   - `rga.ref` is set to `ins_str.ref`.
   - `rga.elems` is set to `ins_str.data`.


## The `ins_bin` Operation

The `ins_bin` operation inerts a binary blob into an existing `bin` node
following the RGA algorithm semantics. The [`ins_bin` operation][ins-bin-op] is
applied to a JSON CRDT document by the following routine:

[ins-bin-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_bin-Operation

1. A `bin` node with ID `ins_bin.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of a `bin` node type, the
   operation is ignored and the application routine terminates.
2. The binary blob `ins_bin.data` is inserted into the `bin` node by the
   `rga.insert()` procedure, where the parameters are:
   - `rga.node` is set to `bin.id`.
   - `rga.ref` is set to `ins_bin.ref`.
   - `rga.elems` is set to `ins_bin.data`.


## The `ins_arr` Operation

The `ins_arr` operation inerts a list of elements into an existing `arr` node
following the RGA algorithm semantics. The [`ins_arr` operation][ins-arr-op] is
applied to a JSON CRDT document by the following routine:

[ins-arr-op]: /specs/json-crdt-patch/patch-document/operations#The-ins_arr-Operation

1. An `arr` node with ID `ins_arr.node` is retrieved from the `model.index`. If
   the node does not exist or the node is not of an `arr` node type, the
   operation is ignored and the application routine terminates.
2. For each element `elem` in `ins_arr.data`, if `elem` is less or equal to `arr.id`,
   the element is ignored, remove it from the insertion list `ins_arr.data`.
3. The list of elements `ins_arr.data` is inserted into the `arr` node by the
   `rga.insert()` procedure, where the parameters are:
   - `rga.node` is set to `arr.id`.
   - `rga.ref` is set to `ins_arr.ref`.
   - `rga.elems` is set to `ins_arr.data`.


## The `del` Operation

The [`del` operation][del-op] is applied to a JSON CRDT document by the
following routine:

[del-op]: /specs/json-crdt-patch/patch-document/operations#The-del-Operation

1. A node with ID `del.node` is retrieved from the `model.index`. If the node
   does not exist, the operation is ignored and the application routine
   terminates.
2. If the node `n` is not of a `str`, `bin`, or `arr` node type, the operation
   is ignored and the application routine terminates.
3. In node `n` mark all elements with IDs in `del.list` as deleted.


## The `nop` Operation

All `nop` operations are silently ignored.


---

# JSON CRDT > Encoding

The JSON CRDT `model` can be serialized for storage or transmission. Multiple
serialization formats are supported, which allows to server different use cases.

Two major serialization approaches are: (1) *structural*; and (2) *indexed*.
Just like JSON CRDT model organizes CRDT nodes by structure or by index, the
serialization codecs can also organize the data by structure or by index.

The structural codecs store the data in a tree-like structure, the tree follows
the structure of the JSON CRDT model and view. The indexed encoding format
stores the data in a flat map, where each node is identified by its index in
the map. The indexed approach allows to store and retrieve each document node
independently, for example, it allows to only read and write the CRDT nodes
that need to be modified.

Finally, there is the *sidecar* format, which is also a structural encoding,
but it encodes only the metadata, the `model.view` is stored separately. This
approach allows to encode the view as plain JSON or CBOR document and store
the CRDT metadata separately. This approach is useful when the viewer is
only interested in the read-only view of the document and does not need
to implement the JSON CRDT specification.


---

# JSON CRDT > Encoding > Structural Encoding

Structural serialization docs store the data in a tree-like structure, the tree
follows the structure of the JSON CRDT model and view. In this encoding format
small pieces of CRDT metadata are added to each node, which allows to
reconstruct the CRDT model and view from the serialized data.


---

# JSON CRDT > Encoding > Structural Encoding > Verbose Structural Format

The `verbose` encoding format specifies how JSON CRDT model is serialized into
a human-readable JSON-like objects. This format consumes the most space, but is
the easiest to read and debug.

~~~jj.note
The resulting JSON-like objects do not have to be encoded as JSON, other
JSON-like serializers---such as CBOR or MessagePack---can be used as well.
~~~


## Document Encoding

The document starts with the `model` encoding, which is a JSON object with the
following properties:

- `"time"` --- clock vector, which contains the latest values of all known
  logical clocks of other peers. The first clock is the local clock of this
  document, the remaining clocks are the clocks of other peers. Each clock is
  encoded as a timestamp, see below.
- `"root"` --- contains encoding of the CRDT node to which the `model.root`
  property points to. The root node is the entry point to the JSON CRDT
  document. See below, *Node Encoding*.


## Timestamp Encoding

Timestamps are represented as array 2-tuples of: (1) session ID, and
(2) sequence number, e.g. `[123, 456]`.


## RGA Chunk Tombstone Encoding

Chunk tombstones are encoded as JSON objects with the following properties:

- `"id"` --- a timestamp, which is the ID of the deleted chunk.
- `"span"` --- a number, which is the number of elements in the deleted chunk.


## Node Encoding

Each JSON CRDT node is encoded as a JSON-like object. Every node has a `"type"`
property, which specifies the type of the node. The `"type"` property is a
string, which is set to one of the following values: `"con"`, `"val"`, `"obj"`,
`"vec"`, `"str"`, `"bin"`, or `"arr"`. Every node also has an `"id"` property,
which is a timestamp, which is the ID of the node.

If a node has a reference to another node, then the reference is inline in the
node. For example, the `val` node has a `"value"` property, which is a reference
to another node. The `val` node is encoded as a JSON object with the child node
inlined in its `"value"` property.


### The `con` Node

The `con` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"con"`.
- `"id"` --- a timestamp, which is the ID of the `con` node.
- `"value"` --- an optional JSON/CBOR value, which is the value of
  the `con` node when it is not `undefined` and is not a timestamp.
- `"timestamp"` --- an optional timestamp, which is the value of the
  `con` node when it is a timestamp.

When the raw value of the `con` node is `undefined`, both, the `"value"` and
`"timestamp"` properties are omitted.

Example where the node value is a primitive value `123`:

```json
{
  "type": "con",
  "id": [123, 456],
  "value": 123
}
```

Example where the node value is a JSON object:

```json
{
  "type": "con",
  "id": [123, 456],
  "value": {
    "foo": "bar",
    "baz": 123
  }
}
```

Example where the node value is a timestamp:

```json
{
  "type": "con",
  "id": [123, 456],
  "timestamp": [123, 100]
}
```

Example where the node value is `undefined`:

```json
{
  "type": "con",
  "id": [123, 456]
}
```


### The `val` Node

The `val` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"val"`.
- `"id"` --- a timestamp, which is the ID of the `val` node.
- `"value"` --- the value of this `val` node, which is another CRDT node.

For example:

```json
{
  "type": "val",
  "id": [123, 456],
  "value": {
    "type": "con",
    "id": [123, 100],
    "value": 123
  }
}
```


### The `obj` Node

The `obj` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"obj"`.
- `"id"` --- a timestamp, which is the ID of the `obj` node.
- `"map"` --- a JSON object, which maps keys to values, where each key is a
  string and each value is a CRDT node.

For example:

```json
{
  "type": "obj",
  "id": [123, 456],
  "map": {
    "foo": {
      "type": "con",
      "id": [123, 100],
      "value": true
    },
    "bar": {
      "type": "con",
      "id": [123, 101],
      "value": false
    }
  }
}
```


### The `vec` Node

The `vec` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"vec"`.
- `"id"` --- a timestamp, which is the ID of the `vec` node.
- `"map"` --- a JSON array, which maps indexes to values, where each index is a
  non-negative integer and each value is a CRDT node. If the vector is sparse,
  then the gaps are filled with `null`.

For example:

```json
{
  "type": "vec",
  "id": [123, 456],
  "map": [
    {
      "type": "con",
      "id": [123, 100],
      "value": true
    },
    null,
    {
      "type": "con",
      "id": [123, 101],
      "value": false
    }
  ]
}
```


### The `str` Node

The `str` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"str"`.
- `"id"` --- a timestamp, which is the ID of the `str` node.
- `"chunks"` --- a sorted JSON array of string chunks or chunk tombstones.

Each string chunk is encoded as a JSON object with the following properties:

- `"id"` --- a timestamp, which is the ID of the chunk, the ID of the first
  element in the chunk.
- `"value"` --- a string, which is the raw text value of the chunk.

For example:

```json
{
  "type": "str",
  "id": [123, 456],
  "chunks": [
    {
      "id": [123, 100],
      "value": "foo"
    },
    {
      "id": [123, 104],
      "span": 4
    }
  ]
}
```


### The `bin` Node

The `bin` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"bin"`.
- `"id"` --- a timestamp, which is the ID of the `bin` node.
- `"chunks"` --- a sorted JSON array of binary chunks or chunk tombstones.

Each binary chunk is encoded as a JSON object with the following properties:

- `"id"` --- a timestamp, which is the ID of the chunk, the ID of the first
  element in the chunk.
- `"value"` --- a string, which is the raw binary value of the chunk, encoded
  as a base64 string.

For example:

```json
{
  "type": "bin",
  "id": [123, 456],
  "chunks": [
    {
      "id": [123, 100],
      "value": "Zm9v"
    }
  ]
}
```


### The `arr` Node

The `arr` node is encoded as a JSON object with the following properties:

- `"type"` --- a string, which is set to `"arr"`.
- `"id"` --- a timestamp, which is the ID of the `arr` node.
- `"chunks"` --- a sorted JSON array of array chunks or chunk tombstones.

Each array chunk is encoded as a JSON object with the following properties:

- `"id"` --- a timestamp, which is the ID of the chunk, the ID of the first
  element in the chunk.
- `"value"` --- a JSON array, a list of CRDT nodes.

For example:

```json
{
  "type": "arr",
  "id": [123, 456],
  "chunks": [
    {
      "id": [123, 102],
      "value": [
        {
          "type": "con",
          "id": [123, 100],
          "value": true
        },
        {
          "type": "con",
          "id": [123, 101],
          "value": false
        }
      ]
    }
  ]
}
```


---

# JSON CRDT > Encoding > Structural Encoding > Compact Structural Format

The `compact` encoding follows the [Compact JSON encoding scheme](/specs/compact-json)
which encodes entities as JSON arrays with a special first element that
represents the type of the entity. This results in a very compact
representation of the document, while still being JSON and human-readable.

~~~jj.note
Documents encoded using the `compact` format can be serialized to a very compact
binary form using binary JSON encoders, such as CBOR or MessagePack.
~~~


## Document Encoding

The document starts with the `model` encoding, which is a JSON array, which is
a 2-tuple. The first element encodes the document's clock table, see below. The
second element encodes the root node, see below.


## Clock Table Encoding

The clock table encodes a vector of logical clocks. The first clock is the
local clock of this document, the remaining clocks are the clocks of other
peers.

The clock table is a flat JSON array of numbers. Every two consecutive numbers
represent a single logical clock. The first number is the session ID, the second
number is the sequence number.


## Timestamp Encoding

Timestamps are represented as array 2-tuples of: (1) session index, and
(2) sequence number difference.

The session index is an index into the clock table, which identifies the
session ID. The session index is stored as a negative number, for example, if
the given session has index of `2` in the clock table, then the session index is
stored as `-2`.

The sequence number difference is the difference between the sequence number of
the given timestamp and the sequence number of the clock in the clock table.


## RGA Chunk Tombstone Encoding

RGA chunk tombstones are encoded as JSON arrays with two elements:

1. a timestamp, which is the ID of the deleted chunk.
2. a number, which is the number of elements in the deleted chunk, the chunk
   span.


## Node Encoding

Each JSON CRDT node is encoded as a JSON array. Every node has a type, which is
the first element of the array, represented by an integer. Every node also has
an ID, which is the second element of the array, represented by a timestamp.

Node types follow `new` operation opcodes of the JSON CRDT Patch operations.
Below table lists the node types and their corresponding type codes. The numbers
are in binary and decimal in parentheses.

```
+==========================+
| Node type  | Code        |
+==========================+
| con        | 000 (0)     |
| val        | 001 (1)     |
| obj        | 010 (2)     |
| vec        | 011 (3)     |
| str        | 100 (4)     |
| bin        | 101 (5)     |
| arr        | 110 (6)     |
+------------+-------------+
```


### The `con` Node

The `con` node is encoded as a JSON array with the following elements:

1. The first element is number `0`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `con` node.
3. The third element, and possibly, the fourth element, represent the value of
   the `con` node.

Depending on the type of value the `con` node holds, the following rules apply:

- If the value is JSON/CBOR value, then the third element is the value. And
  there is no fourth element.
- If the value is a logical timestamp, then the third element is set to `0` and
  the logical timestamp is inserted as the fourth element. The fact that the
  node has four elements with `0` as the third element indicates that the value
  is a logical timestamp.
- If the is `undefined`, then, both, the third and the fourth elements are set
  to `0`.

Example where the node value is a primitive value `123`:

```json
[0, [123, 456], 123]
```

Example where the node value is a JSON object:

```json
[0, [123, 456], {"foo": "bar", "baz": 123}]
```

Example where the node value is a timestamp:

```json
[0, [123, 456], 0, [123, 100]]
```

Example where the node value is `undefined`:

```json
[0, [123, 456], 0, 0]
```


### The `val` Node

The `val` node is encoded as a JSON array with the following elements:

1. The first element is number `1`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `val` node.
3. The third element is the value of this `val` node, which is another CRDT
   node.

For example:

```js
[1, [123, 456],                       // val
  [0, [123, 100], 123]                // con
]
```


### The `obj` Node

The `obj` node is encoded as a JSON array with the following elements:

1. The first element is number `2`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `obj` node.
3. The third element is a JSON object, which maps keys to values, where each
   key is a string and each value is a CRDT node.

For example:

```js
[2, [123, 456],                       // obj
  {
    "foo": [0, [123, 100], 123],      // con
    "bar": [0, [123, 101], "hello"]   // con
  }
]
```


### The `vec` Node

The `vec` node is encoded as a JSON array with the following elements:

1. The first element is number `3`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `vec` node.
3. The third element is a JSON array, which maps indexes to values, where each
   value is a CRDT node. If the vector is sparse, then the gaps are filled with
   `null`.

For example:

```js
[3, [123, 456],                       // vec
  [
    [0, [123, 100], 123],             // con
    null,
    [0, [123, 101], "hello"]          // con
  ]
]
```


### The `str` Node

The `str` node is encoded as a JSON array with the following elements:

1. The first element is number `4`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `str` node.
3. The third element is a JSON array of string chunks or chunk tombstones.

Each string chunk is encoded as a JSON array with the following elements:

1. The first element is a timestamp, which is the ID of the chunk, the ID of
   the first element in the chunk.
2. The second element is a string, which is the raw text value of the chunk.

For example:

```js
[4, [123, 456],                       // str
  [
    [[123, 100], "foo"]               // chunk
    [[123, 103], 4]                   // tombstone
  ]
]
```


### The `bin` Node

The `bin` node is encoded as a JSON array with the following elements:

1. The first element is number `5`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `bin` node.
3. The third element is a JSON array of binary chunks or chunk tombstones.

Each binary chunk is encoded as a JSON array with the following elements:

1. The first element is a timestamp, which is the ID of the chunk, the ID of
   the first element in the chunk.
2. The second element is a binary string (as per CBOR), which is the raw binary
   value of the chunk.

For example:

```js
[5, [123, 456],                       // bin
  [
    [[123, 100], 0x666f6f]            // chunk
  ]
]
```


### The `arr` Node

The `arr` node is encoded as a JSON array with the following elements:

1. The first element is number `6`, which represents the node type.
2. The second element is a timestamp, which is the ID of the `arr` node.
3. The third element is a JSON array of array chunks or chunk tombstones.

Each array chunk is encoded as a JSON array with the following elements:

1. The first element is a timestamp, which is the ID of the chunk, the ID of
   the first element in the chunk.
2. The second element is a JSON array, a list of CRDT nodes.

For example:

```js
[6, [123, 456],                       // arr
  [
    [[123, 102],                      // chunk
      [
        [0, [123, 100], 123],         // con
        [0, [123, 101], "hello"]      // con
      ]
    ]
  ]
]
```


---

# JSON CRDT > Encoding > Structural Encoding > Binary Structural Format

The `binary` JSON CRDT encoding is not human-readable, but is the most space
efficient and performant `model` encoding.


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
- JSON values are encoded using [CBOR](https://www.rfc-editor.org/rfc/rfc8949.html).


### CBOR Encoding

CBOR is used to encode raw JSON/CBOR values. CBOR is a binary encoding format
for JSON-like data structures. CBOR values are represented as a
single variable length box in the diagrams:

```
+========+
|  CBOR  |
+========+
```


### `u8` Encoding

`u8` stands for *Unsigned 8-bit Integer*, it is encoded as a single byte.

- `z` --- unsigned 8 bit integer

```
+--------+
|zzzzzzzz|
+--------+
```


### `u32` Encoding

`u32` stands for *Unsigned 32-bit Integer*, it is encoded as a four byte. The
value follows big-endian order: the high order byte precedes the lower order
byte.

- `z` --- unsigned 32 bit integer

```
+--------+--------+--------+--------+
|zzzzzzzz|zzzzzzzz|zzzzzzzz|zzzzzzzz|
+--------+--------+--------+--------+
```


### `u3u5` Encoding

`u3u5` stands for *Unsigned 3-bit Integer and Unsigned 4-bit Integer*. It
allows to encode two integers in a single byte.

- `x` — the first, 3-bit, unsigned integer
- `y` — the second, 5-bit, unsigned integer

```
+--------+
|xxxyyyyy|
+--------+
```

The `u3u5` values are represented in diagrams as a single fixed length box:

```
+--------+
|  u3u5  |
+--------+
```


### `b1u3u4` Encoding

`b1u3u4` stands for *1-bit Boolean, Unsigned 3-bit Integer, and Unsigned 4-bit
Integer*. It allows to encode a boolean and two integers in a single byte.

- `f` — the boolean flag
- `x` — the first, 3-bit, unsigned integer
- `y` — the second, 4-bit, unsigned integer

```
+--------+
|fxxxyyyy|
+--------+
```

The `b1u3u4` values are represented in diagrams as a single fixed length box:

```
+--------+
| b1u3u4 |
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


## Document Structure

The `model` is encoded as a binary blob, which has thw following parts:

1. First, a `u32` value is encoded, which is the offset of the clock table
   in bytes after the `u32`. The maximum value of the offset is 2147483647,
   which allows for around 2GB of data before the clock table.
2. It is followed by the root node value encoded as a node. See below, *Node
   Encoding*.
3. Lastly, the document is finished with the clock table, which is a list of
   logical clocks. See below, *Clock Table Encoding*.

```
 Clock table offset                  Root node value
 |                                   |
 |                                   |        Clock table
 |                                   |        |
+--------+--------+--------+--------+========+========+
|                u32                |  node  | clocks |
+--------+--------+--------+--------+========+========+
```


## Clock Table Encoding

The clock table is encoded as a list of logical clocks. First, the length of
the clock table is encoded as a `vu57` integer. Then, the clocks are encoded
one after another.

```
 Length
 |        Clock 1   Clock 2   Other clocks
 |        |         |         |
+========+=========+=========+........+
|  vu57  |  clock  |  clock  |        |
+========+=========+=========+........+
```

The clock table is a list of logical clocks. The first clock is the local clock
of this document, the remaining clocks are the clocks of other peers.

Each clock is encoded as two `vu57` integers, where the first integer encodes
the session ID, and the second integer encodes the logical time sequence number.

```
 Session ID
 |        Logical time
 |        |
+========+========+
|  vu57  |  vu57  |
+========+========+
```


## Timestamp Encoding

All known peer clocks are encoded in a clock table. The timestamps in the
document are encoded as a difference between the timestamp and the clock in the
clock table with the same session ID.

Timestamps are encoded as a 2-tuple of: (1) session index, and (2) sequence
number difference. The session index is the ID of the clock with the same
session ID in the clock table. The sequence number difference is the difference
between the sequence number of the given timestamp and the sequence number of
the clock in the clock table.

Timestamps where session index is less than 8 and sequence number difference is
less than 16 are encoded as a single byte using the `b1u3u4` encoding. The bit
flag is set to `0`, the first 3-bit integer is the session index, and the
second 4-bit integer is the sequence number difference.

```
+========+
| b1u3u4 |
+========+
```

All other timestamps are encoded using a `b1vu56` immediately followed by a
`vu57`. The `b1vu56` encodes the session index and the `vu57` encodes the
sequence number difference.

```
+========+========+
| b1vu56 |  vu57  |
+========+========+
```

Timestamp values are represented in diagrams as a single variable length box:

```
+========+
|   id   |
+========+
```


## Node Encoding

Each node consists of the node header and the node value. The node header is
encoded the same way for all nodes. The node value is the custom part of the
node, which is encoded differently for each node type.

```
 Node header
 |
 |        Node payload
 |        |
 |        |
+========+=========+
| header |  value  |
+========+=========+
```

Node types follow `new` operation opcodes of the JSON CRDT Patch specification.
Below table lists the node types and their corresponding type codes. The numbers
are in binary and decimal in parentheses.

```
+==========================+
| Node type  | Code        |
+==========================+
| con        | 000 (0)     |
| val        | 001 (1)     |
| obj        | 010 (2)     |
| vec        | 011 (3)     |
| str        | 100 (4)     |
| bin        | 101 (5)     |
| arr        | 110 (6)     |
+------------+-------------+
```


### Node Header Encoding

Operation starts with the *node header*, which is two or more bytes long. The
node header encodes the node ID, the node type, and the length of the node
value.

- `id` --- node ID, a logical timestamp.
- `TL` --- node type and length.
  - `c` --- node type.
  - `e` --- length of the node value.

```
 Node ID
 |
 |        Node type and length
 |        |
+========+========+
|   id   |   TL   |
+========+========+
```

The encoding of `TL` depends on the value of the length `e`. If length `e` is
less than 31, then the type and length are encoded in a single byte. Otherwise
the type is encoded as the first byte, and the length is encoded as a `vu57`.

When length `e` is less than 31, the first 3 bits of `TL` enccode the node
type `c` and the remaining 5 bits encode the length `e`.

```
          Type
          |
 ID       |  Length
 |        |  |
+========+--------+
|   id   |ccceeeee|
+========+--------+
```

When length is 31 or greater, the first byte encodes the node type `c`, and the
remaining bits are set to `1`. The length is encoded as a `vu57` integer.


```
 ID       Type     Length
 |        |        |
+========+--------+========+
|   id   |ccc11111|  vu57  |
+========+--------+========+
```


### The `con` Node

The `con` node is encoded as a node header, followed by the node value. The node
type `c` is set to `000` (0). The length `e` is set to 0, unless the node value
is a logical timestmap. If the node value is a logical timestamp, then the
length `e` is set to 1.

When the node holds a JSON/CBOR-like value or `undefined`:

```
          Type (000)
          |
 ID       |  Length (0)
 |        |  |
 |        |  |     Value
 |        |  |     |
+========+---|----+========+
|   id   |00000000|  CBOR  |
+========+--------+========+
```

When the node holds a logical timestamp:

```
          Type (000)
          |
 ID       |  Length (1)
 |        |  |
 |        |  |     Timestamp
 |        |  |     |
+========+---|----+========+
|   id   |00000001|   id   |
+========+-------^+========+
```


### The `val` Node

The `val` node is encoded as a node header, followed by the node value. The node
type `c` is set to `001` (1). The length `e` is set to 0.

The header is immediately by the node value, which is another CRDT node.

```
          Type (001)
          |
 ID       |  Length (0)
 |        |  |
 |        |  |     Value
 |        |  |     |
+========+---|----+========+
|   id   |00100000|  node  |
+========+--------+========+
```


### The `obj` Node

The `obj` node is encoded as a node header, followed by the node value. The node
type `c` is set to `010` (2). The length `e` is set to the number of key-value
pairs in the node value. 

```
          Type and length
 ID       |  
 |        |         Values
 |        |         |
+========+---======+===================+
|   id   |010  TL  |  Key-value pairs  |
+========+---======+===================+
```

The node value is a list of key-value pairs. Each key-value pair is encoded as
a key encoded as a CBOR string, followed by a value, which is another CRDT
node.

```
 Key-value pair 1
 |
 |                 Key-value pair 2
 |                 |
+========+========+========+========+........+
|  CBOR  |  node  |  CBOR  |  node  |        |
+========+========+========+========+........+
```


### The `vec` Node

The `vec` node is encoded as a node header, followed by the node value. The node
type `c` is set to `011` (3). The length `e` is set to the number of elements
in the node value.

```
          Type and length
 ID       |  
 |        |         Values
 |        |         |
+========+---======+============+
|   id   |011  TL  |  Elements  |
+========+---======+============+
```

The node value is a list of elements. Each element is another CRDT node. If
the vector is sparse, then the gaps are filled with zero bytes.

```
 Element 0
 |                 Element 2
 |        Gap      |
 |        |        |        Element 3
 |        |        |        |
+========+--------+========+========+........+
|  node  |00000000|  node  |  node  |        |
+========+--------+========+========+........+
```


### The `str` Node

The `str` node is encoded as a node header, followed by the node value. The node
type `c` is set to `100` (4). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |100  TL  |  Chunks  |
+========+---======+==========+
```

Each chunk is consecutively encoded as a CBOR string. If the chunk is a
tombstone, then it is encoded by `u8` integer followed by `vu57` integer. The
`u8` integer is set to `0`, and the `vu57` integer is set to the span length
of the tombstone.

```
 Chunk 1
 |                 Chunk 2 (tombstone)
 |                 |
 |                 |                          Chunk 3
 |                 |                          |
+========+========+========+--------+========+========+========+........+
|   id   |  CBOR  |   id   |00000000|  vu57  |   id   |  CBOR  |        |
+========+========+========+--------+========+========+========+........+
```


### The `bin` Node

The `bin` node is encoded as a node header, followed by the node value. The node
type `c` is set to `101` (5). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |101  TL  |  Chunks  |
+========+---======+==========+
```

Chunks are consecutively encoded one after another. A chunk begins with its ID,
followed by a `b1vu56` integer, where the flag is truthy if the chunk is a
tombstone. The value of the `b1vu56` integer is the span of the chunk. If the
chunk is not deleted (is not a tombstone) it is followed by contents of the
chunk, a binary blob of length equal to the span of the chunk.

```
 Chunk 1
 |                          Chunk 2 (tombstone)
 |                          |
 |                          |                 Chunk 3
 |                          |                 |
+========+========+========+========+========+========+========+========+........+
|   id   | b1vu56 |  blob  |   id   | b1vu56 |   id   | b1vu56 |  blob  |        |
+========+========+========+========+========+========+========+========+........+
```


### The `arr` Node

The `arr` node is encoded as a node header, followed by the node value. The node
type `c` is set to `110` (6). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |110  TL  |  Chunks  |
+========+---======+==========+
```

Chunks are consecutively encoded one after another. A chunk begins with its
ID, followed by `b1vu56` integer, where the flag is truthy if the chunk is
a tombstone. The value of the `b1vu56` integer is the span of the chunk.
If the chunk is not deleted (is not a tombstone) it is followed by
contents of the chunk, a list of CRDT nodes of length equal to the
span of the chunk.

```
 Chunk 1
 |                                   Chunk 2 (tombstone)
 |                                   |
 |                                   |                 Chunk 3
 |                                   |                 |
+========+========+========+========+========+========+========+........+
|   id   | b1vu56 |  node  |  node  |   id   | b1vu56 |   id   |        |
+========+========+========+========+========+========+========+........+
```


---

# JSON CRDT > Encoding > Indexed Encoding

Indexed encoding allows to store the JSON CRDT model in a flat map, where each
node is identified by its index in the map. This allows to store the JSON CRDT
model in a database, which supports only key-value storage. Each node of the
JSON CRDT model is stored as a separate key-value pair in the database.

The Indexed Encoding format borrows many data representation primitives from the
Binary Structural Format. The Indexed Encoding format is a binary encoding
format, which produces a map, where keys are strings and values are binary
blobs.

The map of the encoded document consists of the following keys:

- `"c"` --- the clock table, see below.
- `"r"` --- the root node, see below.
- `"<sid>_<seq>"` --- a key for each node in the document. Where `<sid>` is the
  index of the session ID of the node in the clock table, encoded as Base36.
  And `<seq>` is the sequence number of the node ID, encoded as Base36.


## Clock Table Encoding

The clock table is encoded exactly the same as in the Binary Structural Format.


## Root Node Encoding

The root node is encoded as a single timestamp, which is the ID of the root
node value.

```
+========+
|   id   |
+========+
```


## Node Encoding

The node encoding follows the same format as in the Structural Binary Format,
with two exceptions:

- The node ID is removed from the node header.
- The nested nodes are not encoded inline, but instead only their IDs are
  encoded.


### The `con` Node

The `con` node is encoded exactly the same as in the Structural Binary Format
with the exception that the node ID is removed from the header.

When the node holds a JSON/CBOR-like value or `undefined`:

```
 Type (000)
 |
 |  Length (0)
 |  |
 |  |     Value
 |  |     |
+---|----+========+
|00000000|  CBOR  |
+--------+========+
```

When the node holds a logical timestamp:

```
 Type (000)
 |
 |  Length (1)
 |  |
 |  |     Timestamp
 |  |     |
+---|----+========+
|00000001|   id   |
+-------^+========+
```


### The `val` Node

The `val` node is encoded similar to the Structural Binary Format, with the
node ID removed and the node value containing the ID of the nested node,
instead of the nested node itself.

```
 Type (001)
 |
 |  Length (0)
 |  |
 |  |     Value
 |  |     |
+---|----+========+
|00100000|   id   |
+--------+========+
```


### The `obj` Node

The `obj` node is encoded similar to the Structural Binary Format, with the
exception that the node ID is not encoded and that the nested nodes are not
encoded inline, but instead only their IDs are encoded.

```
 Type and length
 |  
 |         Values
 |         |
+---======+===================+
|010  TL  |  Key-value pairs  |
+---======+===================+
```

Key-value pairs are encoded as a key, followed by the ID of the nested node.

```
 Key-value pair 1
 |
 |                 Key-value pair 2
 |                 |
+========+========+========+========+........+
|  CBOR  |   id   |  CBOR  |   id   |        |
+========+========+========+========+........+
```


### The `vec` Node

The `vec` node is encoded similar to the Structural Binary Format, with the
exception that the node ID is not encoded and that the nested nodes are not
encoded inline, but instead only their IDs are encoded.

```
 Type and length
 |  
 |         Values
 |         |
+---======+============+
|011  TL  |  Elements  |
+---======+============+
```

Elements are encoded as the ID of the nested node.

```
 Element 0
 |                 Element 2
 |        Gap      |
 |        |        |        Element 3
 |        |        |        |
+========+--------+========+========+........+
|   id   |00000000|   id   |   id   |        |
+========+--------+========+========+........+
```


### The `str` Node

The `str` node is encoded similar to the Structural Binary Format, with the
only exception that the node ID is omitted.

```
 Type and length
 |  
 |         Value
 |         |
+---======+==========+
|100  TL  |  Chunks  |
+---======+==========+
```


### The `bin` Node

The `bin` node is encoded similar to the Structural Binary Format, with the
only exception that the node ID is omitted.

```
 Type and length
 |  
 |         Value
 |         |
+---======+==========+
|101  TL  |  Chunks  |
+---======+==========+
```


### The `arr` Node

The `arr` node is encoded similar to the Structural Binary Format, with the
only exceptions that the node ID is omitted and that the nested nodes are not
encoded inline, but instead only their IDs are encoded.

```
 Type and length
 |  
 |         Value
 |         |
+---======+==========+
|110  TL  |  Chunks  |
+---======+==========+
```

Chunks contain node IDs, instead of inlined nodes.

```
 Chunk 1
 |                 Chunk 2 (tombstone)
 |                 |
 |                 |        Chunk 3
 |                 |        |
+========+========+========+========+========+........+
| b1vu56 |   id   | b1vu56 | b1vu56 |   id   |        |
+========+========+========+========+========+........+
```


---

# JSON CRDT > Encoding > Sidecar Encoding

The "sidecar" encoding is an out-of-band encoding of format which allows to
encode the JSON CRDT model metadata separate from the view. This allows to store
the `model.view` separate from the rest of the `model` metadata, which enables
for readers---that do not need the metadata or are not capable to understand
the JSON CRDT metadata---to skip the metadata and read only the view.

The view can be encoded using any codec which supports JSON-like data structure
encoding, like CBOR, JSON, MessagePack, etc. The sidecar encoding is an opaque
blob of metadata, which can be stored next to---but separate---from the view.

```
+========+  +===========+
|  View  |  |  Sidecar  |
+========+  +===========+
```

The Sidecar Encoding is similar to Structural Binary Encoding, with few
differences:

- All raw data, like `con` node values, `str` chunks, `bin` chunks, and `obj`
  keys are not encoded in the sidecar. Instead they are hydrated from the view.
- Object keys are sorted lexicographically before the `obj` nodes are encoded
  and decoded.


## Document Structure

Just like in the Structural Binary Format, the `model` is encoded as a binary
blob, which contains: (1) the clock table offset, (2) the root node, and (3)
the clock table at the end of the document.

```
 Clock table offset                  Root node value
 |                                   |
 |                                   |        Clock table
 |                                   |        |
+--------+--------+--------+--------+========+========+
|                u32                |  node  | clocks |
+--------+--------+--------+--------+========+========+
```


## Clock Table Encoding

The clock table is encoded exactly the same as in the Binary Structural Format.


## Root Node Encoding

The value of the root `val` node `0.0` is encoded as a node, see below.


## Node Encoding

The node encoding follows the same format as in the Structural Binary Format,
with the exception that the raw data, which is present in the view of the model
is skipped.

Each node still has a node header, which encodes the node ID, the node type,
and the length of the node value. The node value is also encoded, but with the
following differences with the Structural Binary Format:

- The `con` node value is omitted.
- The `str` node chunks omit the raw data.
- The `bin` node chunks omit the raw data.
- The `obj` node key strings are omitted.
- The `obj` node keys are sorted lexicographically before the node is encoded.


### The `con` Node

The `con` node is encoded the same as in the Binary Structural Format, with the
exception that the raw JSON/CBOR value is omitted.

When the node holds a JSON/CBOR-like value or `undefined`:

```
          Type (000)
          |
 ID       |  Length (0)
 |        |  |
 |        |  |
 |        |  |
+========+---|----+
|   id   |00000000|
+========+--------+
```

When the node holds a logical timestamp:

```
          Type (000)
          |
 ID       |  Length (1)
 |        |  |
 |        |  |     Timestamp
 |        |  |     |
+========+---|----+========+
|   id   |00000001|   id   |
+========+-------^+========+
```

When the `con` node's value is a logical timestamp, the value in the view
is encoded as `null`.


### The `val` Node

The `val` node is encoded exactly as in the Binary Structural Format.


### The `obj` Node

The `obj` node is encoded as a node header, followed by the node value. The node
type `c` is set to `010` (2). The length `e` is set to the number of key-value
pairs in the node value. 

```
          Type and length
 ID       |  
 |        |         Values
 |        |         |
+========+---======+===============+
|   id   |010  TL  |  Values only  |
+========+---======+===============+
```

Key-value pairs paris are first sorted lexicographically by the key, and then
only the values are encoded. Each value is encoded as another CRDT node.

```
 Value 1
 |
 |        Value 2
 |        |
+========+========+........+
|  node  |  node  |        |
+========+========+........+
```


### The `vec` Node

The `vec` node is encoded as a node header, followed by the node value. The node
type `c` is set to `011` (3). The length `e` is set to the number of elements
in the node value.

```
          Type and length
 ID       |  
 |        |         Values
 |        |         |
+========+---======+============+
|   id   |011  TL  |  Elements  |
+========+---======+============+
```

The node value is a list of elements. Each element is another CRDT node. If
the vector is sparse, then the gaps are filled with `con` nodes with ID `0.0`.

```
 Element 0
 |                  Element 2
 |        Gap       |
 |        |         |        Element 3
 |        |         |        |
+========+=========+========+========+........+
|  node  | con 0.0 |  node  |  node  |        |
+========+=========+========+========+........+
```


### The `str` Node

The `str` node is encoded as a node header, followed by the node value. The node
type `c` is set to `100` (4). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |100  TL  |  Chunks  |
+========+---======+==========+
```

Each chunk is consecutively encoded as a chunk ID followed by `b1vu56` integer.
Where the boolean flat is set to `1` if the chunk is a tombstone, and `0`,
otherwise. The 56-bit integer of the `b1vu56` is set to the span length of the
chunk.

```
 Chunk 1
 |                 Chunk 2
 |                 |
+========+========+========+========+........+
|   id   | b1bu56 |   id   | b1bu56 |        |
+========+========+========+========+........+
```


### The `bin` Node

The `bin` node is encoded as a node header, followed by the node value. The node
type `c` is set to `101` (5). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |101  TL  |  Chunks  |
+========+---======+==========+
```

Each chunk is consecutively encoded as a chunk ID followed by `b1vu56` integer.
Where the boolean flat is set to `1` if the chunk is a tombstone, and `0`,
otherwise. The 56-bit integer of the `b1vu56` is set to the span length of the
chunk.

```
 Chunk 1
 |                 Chunk 2
 |                 |
+========+========+========+========+........+
|   id   | b1bu56 |   id   | b1bu56 |        |
+========+========+========+========+........+
```


### The `arr` Node

The `arr` node is encoded as a node header, followed by the node value. The node
type `c` is set to `110` (6). The length `e` is set to the number of chunks in
the node.

```
          Type and length
 ID       |  
 |        |         Value
 |        |         |
+========+---======+==========+
|   id   |110  TL  |  Chunks  |
+========+---======+==========+
```

Each chunk is consecutively encoded as a chunk ID followed by `b1vu56` integer.
Where the boolean flat is set to `1` if the chunk is a tombstone, and `0`,
otherwise. The 56-bit integer of the `b1vu56` is set to the span length of the
chunk. Finally, after `b1vu56` follows the list of nodes, which are the elements
of the chunk.

```
 Chunk 1
 |
+========+========+========+========+........+
|   id   | b1bu56 |  node  |  node  |        |
+========+========+========+========+........+
```


---

# JSON CRDT > License

json-joy JSON CRDT specification is licensed under the
[__Attribution-ShareAlike 4.0 International__ (CC BY-SA 4.0)][license] license.

[![](https://i.creativecommons.org/l/by-sa/4.0/88x31.png)][license]

[license]: https://creativecommons.org/licenses/by-sa/4.0/


