# Project pulse

Demo showing how to listen to database changes and send them to a WebSocket connection, akin to FireBase, Supabase and etc. 

```bash
$ '/ws/all' -> Listen to all tables + all rows
$ '/ws/$table' -> Listen to all events on a specific table
$ '/ws/$table/$id' -> Listen to all events on a specific table + specific row.
```

## Limitations

1. `$id` can only match the rows that do contain that.
2. On transactions, events are pushed batched after the commit.
3. It's just a demo. Not a real service.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## MakeFile

run all make commands with clean tests
```bash
make all build
```

build the application
```bash
make build
```

run the application
```bash
make run
```

live reload the application
```bash
make watch
```

run the test suite
```bash
make test
```

clean up binary from the last build
```bash
make clean
```
