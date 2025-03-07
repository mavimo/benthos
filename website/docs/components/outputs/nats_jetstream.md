---
title: nats_jetstream
type: output
status: experimental
categories: ["Services"]
---

<!--
     THIS FILE IS AUTOGENERATED!

     To make changes please edit the contents of:
     lib/output/nats_jetstream.go
-->

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

:::caution EXPERIMENTAL
This component is experimental and therefore subject to change or removal outside of major version releases.
:::
Write messages to a NATS JetStream subject.

Introduced in version 3.46.0.


<Tabs defaultValue="common" values={[
  { label: 'Common', value: 'common', },
  { label: 'Advanced', value: 'advanced', },
]}>

<TabItem value="common">

```yml
# Common config fields, showing default values
output:
  label: ""
  nats_jetstream:
    urls: []
    subject: ""
    max_in_flight: 1024
```

</TabItem>
<TabItem value="advanced">

```yml
# All config fields, showing default values
output:
  label: ""
  nats_jetstream:
    urls: []
    subject: ""
    max_in_flight: 1024
    tls:
      enabled: false
      skip_cert_verify: false
      enable_renegotiation: false
      root_cas: ""
      root_cas_file: ""
      client_certs: []
    auth:
      nkey_file: ""
      user_credentials_file: ""
```

</TabItem>
</Tabs>

### Authentication

There are several components within Benthos which utilise NATS services. You will find that each of these components
support optional advanced authentication parameters for [NKeys](https://docs.nats.io/nats-server/configuration/securing_nats/auth_intro/nkey_auth)
and [User Credentials](https://docs.nats.io/developing-with-nats/security/creds).

An in depth tutorial can be found [here](https://docs.nats.io/developing-with-nats/tutorials/jwt).

#### NKey file

The NATS server can use these NKeys in several ways for authentication. The simplest is for the server to be configured
with a list of known public keys and for the clients to respond to the challenge by signing it with its private NKey
configured in the `nkey_file` field.

More details [here](https://docs.nats.io/developing-with-nats/security/nkey).

#### User Credentials file

NATS server supports decentralized authentication based on JSON Web Tokens (JWT). Clients need an [user JWT](https://docs.nats.io/nats-server/configuration/securing_nats/jwt#json-web-tokens)
and a corresponding [NKey secret](https://docs.nats.io/developing-with-nats/security/nkey) when connecting to a server
which is configured to use this authentication scheme.

The `user_credentials_file` field should point to a file containing both the private key and the JWT and can be
generated with the [nsc tool](https://docs.nats.io/nats-tools/nsc).

More details [here](https://docs.nats.io/developing-with-nats/security/creds).

## Fields

### `urls`

A list of URLs to connect to. If an item of the list contains commas it will be expanded into multiple URLs.


Type: `array`  

```yml
# Examples

urls:
  - nats://127.0.0.1:4222

urls:
  - nats://username:password@127.0.0.1:4222
```

### `subject`

A subject to write to.
This field supports [interpolation functions](/docs/configuration/interpolation#bloblang-queries).


Type: `string`  

```yml
# Examples

subject: foo.bar.baz

subject: ${! meta("kafka_topic") }

subject: foo.${! json("meta.type") }
```

### `max_in_flight`

The maximum number of messages to have in flight at a given time. Increase this to improve throughput.


Type: `int`  
Default: `1024`  

### `tls`

Custom TLS settings can be used to override system defaults.


Type: `object`  

### `tls.enabled`

Whether custom TLS settings are enabled.


Type: `bool`  
Default: `false`  

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

### `auth`

Optional configuration of NATS authentication parameters.


Type: `object`  

### `auth.nkey_file`

An optional file containing a NKey seed.


Type: `string`  

```yml
# Examples

nkey_file: ./seed.nk
```

### `auth.user_credentials_file`

An optional file containing user credentials which consist of an user JWT and corresponding NKey seed.


Type: `string`  

```yml
# Examples

user_credentials_file: ./user.creds
```


