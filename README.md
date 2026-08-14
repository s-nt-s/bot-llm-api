# bot-api

Servicio HTTP en Go para recibir consultas dirigidas a bots locales y enrutar solicitudes a proveedores de IA.

## Requisitos

- Go (versión indicada en `go.mod`).

## Variables de entorno relevantes

- `ADDRESS` — dirección donde escucha el servidor. Valor por defecto: `127.0.0.1:8080`.
- `GEMINI` — lista de claves API para el proveedor Gemini. Debe ser una lista separada por espacios (ej: `key1 key2`). Cada clave registrada crea una instancia del proveedor.
- `BOT_HISTORY_MAX_MESSAGES` — máximo de mensajes en el historial por conversación (por defecto `100`).
- `BOT_CACHE_MAX_ENTRIES` — máximo de entradas en caché para instancias de bots (por defecto `1000`).
- `TIMEZONE` — zona horaria usada por utilidades internas (por defecto `UTC`).

## Ejecutar

Ejecución rápida en desarrollo:

```bash
go run .
```

Cambiar la dirección de escucha:

```bash
ADDRESS=127.0.0.1:9090 go run .
```

Script de arranque (carga `.env` si existe):

```bash
./run.sh
```

## Endpoints

Rutas disponibles (manejadas en `internal/httpapi/server.go`):

- `GET /{bot}/query?ask=texto` — consulta simple al bot.
- `POST /{bot}/query` — enviar `ask` como form (`application/x-www-form-urlencoded`).
- `GET /{bot}/chat/{user}?ask=texto` — conversación asociada a un usuario.
- `POST /{bot}/chat/{user}` — enviar `ask` como form (`application/x-www-form-urlencoded`).

Notas importantes:

- El servidor solo acepta métodos `GET` y `POST`.
- Para `POST`, el servidor usa `r.ParseForm()` para obtener parámetros; el cuerpo JSON **no** es procesado por defecto. Use `application/x-www-form-urlencoded` o pase `ask` en la query string.
- `ask` es obligatorio y no puede estar vacío. `bot` es obligatorio; `user` es opcional y puede omitirse.
- Las respuestas son JSON y usan la estructura `config.Message` con campos como `reply`, `status`, `error` y `model`.

Ejemplo con `curl` (GET):

```bash
curl "http://127.0.0.1:8080/mybot/query?ask=Hola"
```

Ejemplo con `curl` (POST form):

```bash
curl -X POST -d "ask=Hola" "http://127.0.0.1:8080/mybot/query"
```

## Bots y usuarios (formato de archivos)

- Los archivos de configuración se cargan desde el directorio `bot/`.
- Cada bot debe definir su configuración en `bot/<bot>/_.md`. El front matter YAML proporciona los campos (p. ej. `name`) y el cuerpo Markdown se usa como `profile`.
- Cada usuario opcional debe definirse en `bot/<bot>/<user>.md` con front matter YAML (por ejemplo `name`) y el cuerpo como `profile`.

Implementación concreta:

- `internal/config.NewBotConfig` busca `bot/<bot>/_.md`, parsea el front matter YAML para poblar `name` y toma el cuerpo Markdown como `profile`.
- `internal/config.NewUserConfig` hace lo mismo para `bot/<bot>/<user>.md`.

Ejemplo mínimo de `bot/mybot/_.md`:

```
---
name: "Mi Bot"
---
Este texto (Markdown) se considera el `profile` del bot y será usado como prompt del sistema.
```

## Proveedores LLM

- Los proveedores se registran desde `internal/providers`. El repositorio incluye un proveedor Gemini de ejemplo en `internal/providers/gemini.go`.
- Gemini se inicializa automáticamente si `GEMINI` contiene claves API (se crean tantas instancias como claves haya).

## Desarrollo y pruebas

Ejecutar todos los tests:

```bash
go test ./...
```

Formateo:

```bash
gofmt -w .
```

## Contribuir

Abre una issue o PR con cambios. Añade tests para nuevas funcionalidades y ejecuta `go test ./...` antes de enviar.

## Licencia

Ver fichero `LICENSE` en la raíz del repositorio.
