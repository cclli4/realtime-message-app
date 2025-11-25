# Real-Time Chat Application (Distributed)

This project demonstrates an event-driven chat system built with Go + Fiber, WebSockets, and an MQTT broker. Messages are published from the gateway to MQTT topics so multiple services (gateway, persistence worker, future notifiers) can consume and act on them independently.

## Getting Started

1. **Prepare dependencies**
   ```bash
   docker run --rm -p 1883:1883 eclipse-mosquitto:2
   docker run --rm -p 6379:6379 redis:7-alpine
   docker run --rm -p 3307:3306 \
     -e MYSQL_ROOT_PASSWORD=rootpass123 \
     -e MYSQL_DATABASE=message_app \
     -e MYSQL_USER=message_user \
     -e MYSQL_PASSWORD=pass123 \
     mysql:8.0
   docker run --rm -p 27017:27017 mongo:6
   ```
   Adjust `.env` if you host the services elsewhere. When running outside Docker Compose, point `DB_HOST`, `MONGO_URI`, `MQTT_BROKER_URL`, and `REDIS_URL` to `localhost`.

2. **Run the WebSocket/HTTP gateway**
   ```bash
   go run main.go
   ```
   - HTTP UI: `http://localhost:3000`
   - WebSocket server: `ws://localhost:9000/message/v1/send`

3. **Run background workers/services (separate processes/containers)**
   ```bash
   go run ./cmd/persistence-worker
   go run ./cmd/notification-worker
   go run ./cmd/query-service
   ```
   - Persistence worker subscribes to `MQTT_CHAT_TOPIC` and inserts messages into MongoDB.
   - Notification worker listens to the same topic and simulates downstream notifications (log file + stdout).
   - Query service exposes REST endpoints (`GET /messages`, `GET /presence/:user`) powered by MongoDB + Redis.

### Docker

The root `Dockerfile` now builds all binaries (`message-app`, `persistence-worker`, `notification-worker`, `query-service`). Static assets are also packaged via `frontend/Dockerfile` (nginx). You can run them as separate containers:
```bash
docker build -t chat-gateway .
docker run --rm --env-file .env chat-gateway ./message-app
docker run --rm --env-file .env chat-gateway ./persistence-worker
docker run --rm --env-file .env chat-gateway ./notification-worker
docker run --rm --env-file .env -p 3100:3100 chat-gateway ./query-service
docker build -t chat-frontend -f frontend/Dockerfile .
docker run --rm -p 8080:80 chat-frontend
```
Compose/Kubernetes manifests can wire these containers together with Mosquitto/Redis for a full distributed stack.

For local orchestration you can also run:
```bash
docker compose up --build
```
This brings up MySQL (published on host port `3307`), MongoDB, Redis, Mosquitto (with external listener configured via `docker/mosquitto.conf`), scalable gateway replicas (scale via `docker compose up -d --scale gateway=2`), an nginx load balancer on `http://localhost:3000` / `ws://localhost:9000`, persistence worker, notification worker, the query service, and a dedicated nginx frontend on `http://localhost:8082` as independent containers that communicate purely via MQTT/Redis/HTTP/DB.

## Service Map

| Service | URL/Port | Purpose |
|---------|----------|---------|
| `chat-frontend` | `http://localhost:8082` | Static HTML/JS client served by nginx |
| `gateway-lb` | `http://localhost:3000`, `ws://localhost:9000` | Nginx reverse proxy/load balancer forwarding to gateway replicas |
| `gateway` | internal only (`expose 3000/9000`) | Auth + HTTP API + WebSocket publisher/subscriber (scale with `--scale gateway=N`) |
| `chat-query-service` | `http://localhost:3100` | REST history + presence API |
| `chat-mysql` | `localhost:3307` | Relational DB for users/sessions |
| `chat-mongo` | `localhost:27017` | Message history store |
| `chat-redis` | `localhost:6379` | Presence cache |
| `chat-mqtt` | `localhost:1883` / `localhost:9001` | MQTT broker for pub/sub |
| `chat-persistence-worker` | - | Persists MQTT events to Mongo |
| `chat-notification-worker` | - | Consumes MQTT events, logs notifications |
Check `docker compose ps --format "table {{.Service}}\t{{.Status}}\t{{.Publishers}}"` for current status/ports.
