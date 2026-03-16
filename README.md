# Personal API Service

Small production-ready API stack for storing and reading messages.

## Stack

- Go API
- PostgreSQL
- Nginx reverse proxy
- Docker Compose

## Endpoints

- `GET /healthz`
- `POST /message`
- `GET /message?limit=50`

Example create request:

```bash
curl -X POST http://localhost/message \
  -H "Content-Type: application/json" \
  -d '{"text":"hello from phone"}'
```

Example list request:

```bash
curl http://localhost/message
```

## Run locally

1. Copy `.env.example` to `.env` and set a real password.
2. Start Docker Desktop if it is not already running.
3. Start the stack:

```bash
cp .env.example .env
docker compose up --build -d
```

If `.env` is missing, Docker Compose will now stop immediately with a clear error instead of starting PostgreSQL with an empty password.

4. Check health:

```bash
curl http://localhost/healthz
```

5. Smoke test the API:

```bash
curl -X POST http://localhost/message \
  -H "Content-Type: application/json" \
  -d '{"text":"hello from phone"}'

curl http://localhost/message
```

## Why this is production-ready enough

- graceful shutdown in the Go server
- PostgreSQL connection pooling
- startup migration for a single simple schema
- health checks for PostgreSQL, API, and Nginx dependency order
- JSON logging
- request size limit and Nginx rate limiting
- only Nginx is exposed publicly

## Deploy on a server

Install Docker and Docker Compose plugin, then place this folder on the server.

Use a firewall so only ports `80` and `443` are open publicly.

For HTTPS, terminate TLS at Nginx or put a managed proxy in front of it.

## Start on boot with systemd

Example unit file is in `deploy/systemd/personal-api-compose.service`.

Install it on the server, then run:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now personal-api-compose
```