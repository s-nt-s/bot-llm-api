# bot-api

Go HTTP service for receiving bot queries.

## Ejecutar

```bash
go run .
```

The service listens on `:8080` by default. You can change it with `ADDRESS`, for example `ADDRESS=:9090 go run .`.

## Rutas

- `GET /{bot}/query?ask=texto`
- `POST /{bot}/query` with `ask` in a form (`application/x-www-form-urlencoded`) or JSON (`{"ask":"text"}`)
- `GET /{bot}/chat/{user}?ask=texto`
- `POST /{bot}/chat/{user}` with `ask` in a form or JSON

`bot`, `user`, and `ask` are required and cannot be empty. A bot must have a valid `bot/{bot}/0.yaml` file with non-empty string fields `name` and `profile`. A user must have a valid `bot/{bot}/{user}.yaml` file with non-empty string fields `name` and `profile`. Successful responses are JSON containing the received values.
