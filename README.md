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
| dbpath | File path to store DB data | /tmp/badger |
| nats | URL of NATS | nats://localhost:4222 |

## API

### Scan Bites

```
GET /conversation/:key/scan
```

### Get Bite

```
GET /conversation/:key/start/:start
```

### Get Bite User

```
GET /conversation/:key/start/:start/user
```
