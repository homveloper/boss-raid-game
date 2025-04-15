---

# JSON Expression

JSON Expression is language for representing computation expressions---which
consist of operators and operands---as compact, but human-readable JSON arrays.


---

# JSON Expression > Overview

This document describes JSON Expression, a JSON-based expression language. JSON
Expression is similar to [S-expression](https://en.wikipedia.org/wiki/S-expression),
but expressed (pun intended) in JSON.

JSON Expression allows to specify a mathematical or logical expression in a
JSON-like value, which can then be evaluated to obtain a result, which itself
is a JSON-like value.

JSON expressions are JSON values themselves, so they can be serialized for
storage or transmission over the network.


## Terminology

- *JSON Expression* is a JSON-based expression language.
- *JSON expression* is one expression in JSON Expression.
- *JSON-like value* is a JSON value, which may include more types than the ones
  defined in the JSON specification. For example, programming languages and
  serialization formats (like CBOR) extend JSON with additional types, such
  as `undefined`, binary data, dates, etc.


---

# JSON Expression > Expressions

A JSON expression is composed of a single root expression. An expression is a
JSON array with the following structure:

```json
[
  "operator-name",
  "operand-1",
  "operand-2",
  ...
]
```

~~~jj.aside
JSON Expression follows [Compact JSON](/specs/compact-json) encoding
scheme.
~~~

The first element of the array is the name of the operator. The remaining
elements are the operands of the operator.

For example, the following expression sums the numbers 1 and 2, the `"+"` is the
operator, and the numbers `1` and `2` are the operands:

```json
[
  "+",
  1,
  2
]
```

In every expression, the first element of the array is the *operator*. The
operator is a literal JSON value that represents the operation to be performed.
The operator is treated as a constant value, and is not evaluated.

The remaining elements of the array are the *operands* of the operator. The
operands are evaluated, and the result of the evaluation is passed to the
operator. Every operand is an expression itself, hence, an expression can be
composed of other expressions.

```json
[
    "operator-name",
    "expression-1",
    "expression-2",
    ...
]
```

For example, the following expression sums the number 1 with the result of the
summation of the numbers 2 and 3:

```json
["+", 1, ["+", 2, 3]]
```


---

# JSON Expression > Evaluation

The evaluation of an expression is performed by first evaluating one or more
operands of the expression, and passing the result of the evaluation to the
operator. Then operator performs some computation on the operands, it may
evaluate more operands, and finally, it returns a JSON-like value as the
result of the evaluation of the expression.

Typically, operands are evaluated in the order they appear in the expression,
from left to right. The result of the evaluation of an operand is a JSON-like
value which is passed to the operator. For example, first `1` is evaluated, then
`2`, and finally `3`:

```json
["+", 1, 2, 3]              // 6
```

Some operators may evaluate their operands in a different order. For example,
the `if` operator evaluates its operands in the following order: first the
condition, then it only evaluates the `then` operand if the condition is true,
otherwise it evaluates the `else` operand. The following expression evaluates
the `else` operand, because the condition is false:

```json
["if", ["==", 1, 2], 3, 4]  // 4
```

The operator performs an atomic computation, and returns a JSON-like
value as the result of the evaluation of the expression. The result of the
evaluation of an expression is always a JSON-like value. Intermediate results
of the evaluation of an operator are not exposed in any way, to other operators
or to the user.

The result of the evaluation of an expression is passed to the operator of the
parent expression recursively, until the root expression is reached. The result
of the evaluation of the root expression is the result of the evaluation of the
whole expression.


---

# JSON Expression > Operators and Operands

JSON Expression operators are classified into three groups based on their arity
(the number of operands they accept):

- Nullary operators: operators that take no operands.
- Positive fixed arity operators: operators that take a pre-defined positive
  number of operands.
- Variadic operators: operators that take one or more indefinite number of
  operands.

A JSON array in the position of an operand is treated as an expression, and is
evaluated before being passed to the operator. All other operand types (such as
numbers, strings, booleans, nulls, objects, etc.) are passed to the operator
as-is.


## Nullary Operators

*Nullary operators* or *constants* are operators that take no operands. They
represent literal values in JSON Expression. I.e. an array with a single
element is a nullary operator, where that single element is the value of the
constant.

For example, a `null` value literal is represented as:

```json
[null]                  // null
```

Booleans, strings, and numbers:

```json
[true]                  // true
["hello"]               // hello
[42]                    // 42
```

Literal recursive types---arrays and objects:

```json
[["hello", "world"]]    // [ "hello", "world" ]
[{"foo": "bar"}]        // { "foo": "bar" }
```

Array literal values must be always represented by a nullary operator
expression. For example:

```json
[[1, 2, 3]]             // [ 1, 2, 3 ]
```

Values of all other types can be represented as-is, without the need for a
nullary operator.

~~~jj.note
Arrays:

```json
/// Only
[[1, 2, 3]]             // [ 1, 2, 3 ]
```

Objects:

```json
{"foo": "bar"}          // { "foo": "bar" }
// or
[{"foo": "bar"}]        // { "foo": "bar" }
```

Strings:

```json
"abc"                   // "abc"
// or
["abc"]                 // "abc"
```

Numbers:

```json
1                       // 1
// or
[1]                     // 1
```

Booleans:

```json
true                    // true
// or
[true]                  // true
```

Nulls:

```json
null                    // null
// or
[null]                  // null
```
~~~


## Positive Fixed Arity Operators

*Positive fixed arity operators* are operators which accept a pre-defined
positive number of operands, i.e. one or more operands. They are typically
known as *unary operators*, *binary operators*, and *ternary operators* for
arities of 1, 2, and 3, respectively.

Expressions of positive fixed arity operators are represented as arrays with
the operator name as the first element, and a fixed number of the remaining
array elements, which are the operands.

For example, a unary negation operator:

```json
["not", true]          // false
```

A binary addition operator:

```json
["+", 1, 2]            // 3
```

A ternary conditional operator:

```json
["if", true, 1, 2]     // 1
```


## Variadic Operators

*Variadic operators* are operators which accept one or more operands, but the
number of operands is not pre-defined, i.e. the number of operands is variable.

Expressions of variadic operators are represented as arrays, where the first
element is the operator name, and the remaining elements are the operands.

For example, a variadic addition operator:

```json
["+", 1, 2, 3]         // 6
```


### Variadic Operator Evaluation

Variadic operators are typically a sugar syntax for intrinsically binary
operators. For example, it is often more convenient to write:

```json
["+", 1, 2, 3]         // 6
```

Instead of:

```json
["+", ["+", 1, 2], 3]  // 6
```

In the case the variadic operator is a sugar syntax for an intrinsically binary
operator, the evaluation of the variadic operator is performed by recursively
applying the binary form of the operator to all of its operands, from left to
right.

For example, the following representations are all equivalent:

```json
["+", 1, 2, 3, 4, 5]
["+", ["+", 1, 2], 3, 4, 5]
["+", ["+", ["+", 1, 2], 3], 4, 5]
["+", ["+", ["+", ["+", 1, 2], 3], 4], 5]
```

~~~jj.note
When encoding the above expressions in CBOR, the shortest form is just 8 bytes,
while the longest form is 17 bytes. The shortest form is not just more human
readable, but also advantageous for constrained space environments, such as
embedded systems or limited size payloads, like authorization tokens.
~~~


---

# JSON Expression > Input and Output

The JSON Expression expressions could be pure functions, which bear no
side-effects on the environment. However, often one will want to use the
expressions to perform some computation based on arbitrary input data, and
produce some output data; but in a safe and controlled manner.

This section describes the mechanisms for passing data to the expression, and
for retrieving back the result of the computation.


## Expression-as-Data

The simplest form of "input" is to realize that each expression is just a
JSON-like data structure, which can be passed to the expression as an operand.
For example, in the following addition expression:

```json
["+", 1, 2]
```

The operands `1` and `2` are just JSON-like data structures, which can be
substituted with the desired expressions just before the evaluation of the
expression.

The downside of this approach is that the expression is not stable---it changes
every time the input data changes. Hence, it is hard---if not impossible---to
cache or compile the expression.


## Input

The most common way to pass input data to the expression is to use an *input*
JSON-like value, often a JSON object. One can then define an accessor operator,
which retrieves the desired input object or parts of it. For example, one can
define the `get` operator, which retrieves specific value from the input object
using the [JSON Pointer][json-pointer] syntax.

[json-pointer]: https://www.rfc-editor.org/rfc/rfc6901

Consider the following input object:

```json
{
  "foo": "bar",
}
```

Given the above input object, the following expression retrieves
the `"bar"` value of the `"foo"` property:

```json
["get", "/foo"]     // "bar"
```


## Output

Each JSON expression evaluates to a JSON-like value, the result of the most
root JSON expression is the result of the entire expression.


## Errors

An evaluation of some operator might result in an error. Say, a division by
zero, or an attempt to access a non-existing property of an object. In such
cases, any further evaluation immediately terminates, and the error is
propagated all the way up to the root expression.

The error value must be a JSON-like value. The result of the entire expression
should contain some indication that the evaluation resulted in an error, and
provide the JSON-like error value itself.

Any expression could potentially result in an error. An implementation could
even define operators which always result in an error, for example, a unary
operator which always throws an exception.

```json
["throw", "Oops!"]
```


---

# JSON Expression > License

json-joy JSON Expression specification is licensed under the
[__Attribution-ShareAlike 4.0 International__ (CC BY-SA 4.0)][license] license.

[![](https://i.creativecommons.org/l/by-sa/4.0/88x31.png)][license]

[license]: https://creativecommons.org/licenses/by-sa/4.0/


---

# JSON Expression > Appendix



---

# JSON Expression > Appendix > Related Work

## JSON Logic Library

[JSON Logic](https://jsonlogic.com/operations.html) is a library implemented in
various programming languages, including JavaScript, PHP, Python, Ruby, Go
Java, .Net, and C++. It defines a set of around 30 operations, grouped into
categories, such as: accessor operations, logic and boolean operations, numeric
operations, array operations, string operations, and miscellaneous.

The general syntax of the operations is

```
{ "<operator>": [ <argument1>, <argument2>, ... ] }
```

where object keys are the operators, and the values are arrays of arguments.


## MongoDB Aggregation Pipeline Operators

The [MongoDB Aggregation Pipeline Operators](https://www.mongodb.com/docs/manual/reference/operator/aggregation/)
specifies few dozen operations, which can be used inside the MongoDB database.

The operations are grouped into categories, such as arithmetic operators, array
operators, bitwise operators, boolean operators, comparison operators,
conditional operators, and data size operators.

The general syntax of the operators is

```
{ <operator>: [ <argument1>, <argument2>, ... ] }
```

where object keys are the operators, and the values are arrays of arguments.


## AWS SNS Subscription Filter Policies

[AWS SNS Subscription Filter Policies](https://docs.aws.amazon.com/sns/latest/dg/sns-subscription-filter-policies.html)
is a DSL based on JSON, which is used to filter messages sent to an SNS topic.

The general form of filtering policies is to define a JSON object with
keys that represent the message attributes that will be matched, and values that
represent the expected values of these attributes. For example, the following
filtering policy matches messages which include all of the following attributes:

- `store` attribute with the value `example_corp`;
- `event` attribute with a value other than `order_cancelled`;
- `customer_interests` attribute with a value of `rugby`, `football`,
  or `baseball`;
- `price_usd` attribute with a value greater than or equal to `100`.

```json
{
   "store": ["example_corp"],
   "event": [{"anything-but": "order_cancelled"}],
   "customer_interests": [
      "rugby",
      "football",
      "baseball"
   ],
   "price_usd": [{"numeric": [">=", 100]}]
}
```


## MathJSON

[Math JSON](https://cortexjs.io/math-json/) is a lightweight, human-readable
interchange format for mathematical notation. Unlike, JSON Expression, MathJSON
is not a programming language, but a data format for mathematical expressions.


---

# JSON Expression > Appendix > Motivation and Use Cases

One of the motivations was to create a simple and lightweight expression
language, which can be executed performantly, securely, and even just-in-time
compiled to native code.

Another motivation was the observation of the many use cases it could have,
JSON Expression can be applied to unlimited number of use cases. Here I list
the few use cases, which motivated the creation of JSON Expression.


## JSON Patch

[JSON Patch](https://datatracker.ietf.org/doc/html/rfc6902/) has
a [`test` operation](https://datatracker.ietf.org/doc/html/rfc6902/#section-4.6),
which test if a value at a given path matches a given value.

```json
{ "op": "test", "path": "/a/b/c", "value": "foo" }
```

The `test` operation could use a JSON Expression to perform any kind of test,
not just equality. For example, the following JSON Expression tests if the
value at the given path is a string, and if it is, it tests if it is equal
to `foo`:

```json
{ "op": "test", "expr":
    ["and", ["is-string", ["get", "/a/b/c"]], ["==", ["get", "/a/b/c"], "foo"]]
}
```


## JSON Predicate

[JSON Predicate](http://www.watersprings.org/pub/id/draft-snell-json-test-01.html)
is a draft specification, which extends the `test` operation of JSON Predicate
specification. It adds support for a few more operators, such
as `contains`, `matches`, `in`, `less`, etc.

The operations perform similar task to that of the `test` operation, they test
if a value at a given path satisfies some condition.

It turns out all JSON Predicate operations can be expressed using JSON
Expression. For example, the following JSON Predicate operation tests if a
string contains a given substring:

```json
{"op": "contains", "path": "/a/b/c", "value": "ABC"}
```

As before, the JSON Expression equivalent could be:

```json
{ "op": "test", "expr": ["contains", ["get", "/a/b/c"], "ABC"]}
```


## JSON Schema

[JSON Schema](https://json-schema.org/) is a schema definition language for
JSON documents. Some types in the JSON Schema language have ability to define
constraints on the values, for example, one can define
a `minimum` and `maximum` value for a number type.

```json
{
    "type": "number",
    "minimum": 0,
    "maximum": 100
}
```

Almost all constraints specified in the JSON Schema language can be expressed
using JSON Expression. For example, the above JSON Schema could be expressed as:

```json
{
    "type": "number",
    "test": ["and", [">=", ["get", ""], 0], ["<=", ["get", ""], 100]]
}
```


## JSON Type Definition

[JSON Type Definition](https://jsontypedef.com/) (JTD) is a format for defining
types for JSON documents. It is similar to JSON Schema, but targets specifically
the features of a type system.

One of the schemas in JTD is the [Discriminator][discriminator] form, which
represents a union of types. To discriminate between the different types,
a `discriminator` property is used, which specifies the type of the value.
Another major restriction is that the discriminant property is always a string,
i.e. the values of `"eventType"` property must be strings:

```json
{
    "discriminator": "eventType",
    "mapping": {
        "USER_CREATED": {
            "properties": {
                "id": { "type": "string" }
            }
        },
        "USER_PAYMENT_PLAN_CHANGED": {
            "properties": {
                "id": { "type": "string" },
                "plan": { "enum": ["FREE", "PAID"]}
            }
        },
        "USER_DELETED": {
            "properties": {
                "id": { "type": "string" },
                "softDelete": { "type": "boolean" }
            }
        }
    }
}
```

The Discriminator type is very limited, as it can only discriminate based on
the value of a single property; and all the types must be objects; and the
property must be a string.

Using a JSON Expression instead, one could discriminate arbitrarily
based on any features of the data. For example, the following JSON Expression
discriminates between two types based on the value of the `"a"` property:

```json
{
    "discriminator": ["if", ["type", ["get", "/a"], "string"], "v2", "v1"],
    "mapping": {
        "v1": {
            "properties": {
                "a": { "type": "float32" }
            }
        },
        "v2": {
            "properties": {
                "a": { "type": "string" }
            }
        }
    }
}
```


## Authorization Policies

In modern applications, authorization policies are often expressed as a set of
conditions applied to a principal-action-target 3-tuple, where: (1) principal
is the user or service invoking the action; (2) action is the operation being
performed; and (3) target is the resource on which the action is performed.

Given the principal-action-target 3-tuple the authorization policy needs to
decide if the principal is allowed to perform the action on the target. Some
authorizations systems use custom JSON DSLs to express the authorization
policies, for example, [AWS IAM Policy][aws-iam-policy]. Other systems use
custom languages, such as [Open Policy Agent][opa].

[aws-iam-policy]: https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html
[opa]: https://www.openpolicyagent.org/docs/latest/policy-language/

JSON Expression can be used to express authorization policies. For example,
given the following principal-action-target 3-tuple:

```json
{
    "principal": "user:alice",
    "action": "read",
    "target": "resource:foo"
}
```

The following JSON Expression could be used to express the authorization policy:

```json
{
    "policy": {
        "allow": ["and",
            ["==", ["get", "/principal"], "user:alice"],
            ["==", ["get", "/action"], "read"],
            ["==", ["get", "/target"], "resource:foo"]
        ]
    }
}
```

The policy itself is a JSON document, which can be stored and edited using
commonly available tooling. The only new part of the system would be the JSON
Expression, which is used to evaluate the policy, or it can be JIT-compiled to
a more efficient form.


## JWT Claims

[JSON Web Token](https://www.rfc-editor.org/rfc/rfc7519.html) (JWT) is a
standard for representing claims securely between two parties, most often used
to authenticate and authorize access to some API. A JWT is a JSON document,
which contains a set of claims. The claims are represented as a set of
key-value pairs.

Sometimes, the claims may contain detailed authorization information, which
can be used to authorize access to some API. For example, the following JWT
contains the `"scope"` claim, which specifies the permissions granted to the
principal:

```json
{
    "iss": "https://authorization-server.example.com/",
    "sub": "5ba552d67",
    "aud": "https://rs.example.com/",
    "exp": 1639528912,
    "iat": 1618354090,
    "scope": "openid profile reademail"
}
```

Instead of using a custom DSL to express the authorization policy, or
maintaining lists of coarse-grained permissions, one could use JSON Expression
to grant access dynamically to specific resources.

For example, the below JSON Expression grants access to the below JWT claim
holder to execute the `read` action on the `resource:foo` resource.

```json
{
    "iss": "https://authorization-server.example.com/",
    "sub": "5ba552d67",
    "aud":   "https://rs.example.com/",
    "exp": 1639528912,
    "iat": 1618354090,
    "allow": ["and",
        ["==", ["get", "/action"], "read"],
        ["==", ["get", "/target"], "resource:foo"]
    ]
}
```


## Event Filtering

Often message queues or other subscription based systems allow to filter events
based on some criteria. As an example, AWS SNS service has a
[filtering feature][sns-filtering], which allows specify custom filtering
logic for each subscription.

[sns-filtering]: https://docs.aws.amazon.com/sns/latest/dg/sns-message-filtering.html

Message filtering logic could be expressed using JSON Expression. Consider
messages of the following [Cloud Event](https://cloudevents.io/) form written
to a message queue.

```json
{
    "specversion" : "1.0",
    "type" : "com.example.someevent",
    "source" : "/mycontext",
    "subject": null,
    "id" : "C234-1234-1234",
    "time" : "2018-04-05T17:31:00Z",
    "comexampleextension1" : "value",
    "comexampleothervalue" : 5,
    "datacontenttype" : "application/json",
    "data" : {
        "appinfoA" : "abc",
        "appinfoB" : 123,
        "appinfoC" : true
    }
}
```

One could write and compile a JSON Expression to efficiently filter out events
of interest, for example, an expression could look like this:

```json
[
  "and",
    ["==", ["get", "/specversion"], "1.0"],
    ["starts", ["get", "/type"], "com.example."],
    ["in",
        ["get", "/datacontenttype"],
        [["application/octet-stream", "application/json"]]
    ],
    ["==", ["=", "/data/appinfoA"], "abc"],
]
```

It would filter out only messages, which satisfy the following criteria:

- `specversion` is equal to `"1.0"`;
- `type` starts with `"com.example."`;
- `datacontenttype` is either `"application/octet-stream"` or `"application/json"`;
- custom metadata in `data.appinfoA` is equal to `"abc"`.

JSON Expression expressions can be JIT-compiled to machine code, which can be
used to efficiently filter up to tens of thousands of events per second on a
single CPU core.


