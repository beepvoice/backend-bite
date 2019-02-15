# backend-bite

Beep backend handling of audio bites. Chopped up words spoken are uploaded into bite to be stored in a scan-able key-value store.  Subscribes to ```new_bite``` and ```new_bite_user``` events from a NATS publisher.

## Quickstart

```
go build && ./backend-bite
```

## Flags

Flags are supplied to the compiled go program in the form ```-flag=stuff```.

| Flag | Description | Default |
| ---- | ----------- | ------- |
| listen | Port number to listen on | 8080 |
| nats | URL of NATS | nats://localhost:4222 |

## API

### Scan Bites

```
GET /conversation/:key/scan
```

Get a list of bite start times within a conversation key and specified timespan.

#### URL Params

| Name | Type | Description |
| ---- | ---- | ----------- |
| key | String | Audio bite's conversation's ID. |

#### Querystring

| Name | Type | Description |
| ---- | ---- | ----------- |
| from | Epoch timestamp | Time to start scanning from |
| to | Epoch timestamp | Time to scan to |

#### Success (200 OK)

```
Content-Type: application/json
```

```
{
  "previous": <Timestamp of bite before <starts>>,
  "starts": [Timestamp, Timestamp...],
  "next": <Timestamp of bite after <starts>>,
}
```

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | Malformed input (from/to not timestamp, key not alphanumeric). |
| 500 | NATs or protobuf serilisation encountered errors. |

---

### Get Bite

```
GET /conversation/:key/start/:start
```

Get a specific ```bite```.

#### URL Params

| Name | Type | Description |
| ---- | ---- | ----------- |
| key | String | Audio bite's conversation's ID. |
| start | Epoch timestamp | Time the audio bite starts. |

#### Success (200 OK)

Raw audio data.

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | start is not an uint/key is not an alphanumeric string/specified bite could not be found |
| 500 | NATs or protobuf serilisation encountered errors. |

---

### Get Bite User

```
GET /conversation/:key/start/:start/user
```

Get a specific ```bite_user```.

#### URL Params

| Name | Type | Description |
| ---- | ---- | ----------- |
| key | String | Audio bite's conversation's ID. |
| start | Epoch timestamp | Time the audio bite starts. |

#### Success (200 OK)

Raw audio data.

#### Errors

| Code | Description |
| ---- | ----------- |
| 400 | start is not an uint/key is not an alphanumeric string/specified bite could not be found |
| 500 | NATs or protobuf serilisation encountered errors. |
