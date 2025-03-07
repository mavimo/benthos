---
title: schema_registry_encode
type: processor
status: experimental
categories: ["Parsing","Integration"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the contents of:
     lib/processor/schema_registry_encode.go
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

:::caution EXPERIMENTAL
This component is experimental and therefore subject to change or removal outside of major version releases.
:::
Automatically encodes and validates messages with schemas from a Confluent Schema Registry service.

Introduced in version 3.58.0.


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
label: ""
schema_registry_encode:
  url: ""
  subject: ""
  refresh_period: 10m
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
label: ""
schema_registry_encode:
  url: ""
  subject: ""
  refresh_period: 10m
  avro_raw_json: false
  tls:
    skip_cert_verify: false
    enable_renegotiation: false
    root_cas: ""
    root_cas_file: ""
    client_certs: []
```

</TabItem>
</Tabs>

Encodes messages automatically from schemas obtains from a [Confluent Schema Registry service](https://docs.confluent.io/platform/current/schema-registry/index.html) by polling the service for the latest schema version for target subjects.

If a message fails to encode under the schema then it will remain unchanged and the error can be caught using error handling methods outlined [here](/docs/configuration/error_handling).

Currently only Avro schemas are supported.

### Avro JSON Format

By default this processor expects documents formatted as [Avro JSON](https://avro.apache.org/docs/current/spec.html#json_encoding) when encoding Avro schemas. In this format the value of a union is encoded in JSON as follows:

- if its type is `null`, then it is encoded as a JSON `null`;
- otherwise it is encoded as a JSON object with one name/value pair whose name is the type's name and whose value is the recursively encoded value. For Avro's named types (record, fixed or enum) the user-specified name is used, for other types the type name is used.

For example, the union schema `["null","string","Foo"]`, where `Foo` is a record name, would encode:

- `null` as `null`;
- the string `"a"` as `{"string": "a"}`; and
- a `Foo` instance as `{"Foo": {...}}`, where `{...}` indicates the JSON encoding of a `Foo` instance.

However, it is possible to instead consume documents in raw JSON format (that match the schema) by setting the field [`avro_raw_json`](#avro_raw_json) to `true`.

## Fields

### `url`

The base URL of the schema registry service.


Type: `string`  

### `subject`

The schema subject to derive schemas from.
This field supports [interpolation functions](/docs/configuration/interpolation#bloblang-queries).


Type: `string`  

```yml
# Examples

subject: foo

subject: ${! meta("kafka_topic") }
```

### `refresh_period`

The period after which a schema is refreshed for each subject, this is done by polling the schema registry service.


Type: `string`  
Default: `"10m"`  

```yml
# Examples

refresh_period: 60s

refresh_period: 1h
```

### `avro_raw_json`

Whether messages encoded in Avro format should be parsed as raw JSON documents rather than [Avro JSON](https://avro.apache.org/docs/current/spec.html#json_encoding).


Type: `bool`  
Default: `false`  
Requires version 3.59.0 or newer  

### `tls`

Custom TLS settings can be used to override system defaults.


Type: `object`  

### `tls.skip_cert_verify`

Whether to skip server side certificate verification.


Type: `bool`  
Default: `false`  

### `tls.enable_renegotiation`

Whether to allow the remote server to repeatedly request renegotiation. Enable this option if you're seeing the error message `local error: tls: no renegotiation`.


Type: `bool`  
Default: `false`  
Requires version 3.45.0 or newer  

### `tls.root_cas`

An optional root certificate authority to use. This is a string, representing a certificate chain from the parent trusted root certificate, to possible intermediate signing certificates, to the host certificate.


Type: `string`  
Default: `""`  

```yml
# Examples

root_cas: |-
  -----BEGIN CERTIFICATE-----
  ...
  -----END CERTIFICATE-----
```

### `tls.root_cas_file`

An optional path of a root certificate authority file to use. This is a file, often with a .pem extension, containing a certificate chain from the parent trusted root certificate, to possible intermediate signing certificates, to the host certificate.


Type: `string`  
Default: `""`  

```yml
# Examples

root_cas_file: ./root_cas.pem
```

### `tls.client_certs`

A list of client certificates to use. For each certificate either the fields `cert` and `key`, or `cert_file` and `key_file` should be specified, but not both.


Type: `array`  

```yml
# Examples

client_certs:
  - cert: foo
    key: bar

client_certs:
  - cert_file: ./example.pem
    key_file: ./example.key
```

### `tls.client_certs[].cert`

A plain text certificate to use.


Type: `string`  
Default: `""`  

### `tls.client_certs[].key`

A plain text certificate key to use.


Type: `string`  
Default: `""`  

### `tls.client_certs[].cert_file`

The path to a certificate to use.


Type: `string`  
Default: `""`  

### `tls.client_certs[].key_file`

The path of a certificate key to use.


Type: `string`  
Default: `""`  


