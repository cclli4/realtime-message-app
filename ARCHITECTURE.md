# Distributed Systems Architecture Documentation
## Real-Time Chat Application (Event-Driven)

### Project Overview
This document describes the architecture and implementation of a Real-Time Chat Application built with Event-Driven Architecture principles, implementing Publish-Subscribe communication paradigm with multi-threading support for concurrent message handling.

---

## 1. Event-Driven Architecture

### Implementation
The system now uses an **external MQTT broker** as the event bus so each service communicates via topics instead of in-process channels.

#### Key Components:

1. **Event Broker (MQTT)**
   - Location: `web_socket/message.go:40-90`
   - Package: `pkg/mqttclient/mqtt.go`
   - Purpose: Provides durable topics (`MQTT_CHAT_TOPIC`) for distributed fan-out. The gateway subscribes to the topic to push updates to WebSocket clients, and workers subscribe to persist/notify.

```go
mqttClient, err := mqttclient.NewClient(...)
if err := mqttClient.Subscribe(mqttTopic, func(..., m mqtt.Message) {
    var payload models.MessagePayload
    json.Unmarshal(m.Payload(), &payload)
    broadcastToClients(payload)
})
```

2. **Event Producers (Web Clients & Gateway)**
   - WebSocket connections send payloads to the gateway, which publishes serialized messages to MQTT topics.
   - Location: `web_socket/message.go:123-145`

```go
if err := mqttClient.PublishJSON(mqttTopic, msg); err != nil {
    log.Printf("Failed to publish message to MQTT: %v", err)
}
```

3. **Event Consumers (Gateway + Workers)**
   - Gateway subscribes to the same topic to broadcast to connected clients.
   - Persistence worker subscribes independently and writes messages to MongoDB.
   - Location: `web_socket/message.go:60-90`, `cmd/persistence-worker/main.go:33-73`

---

## 2. Publish-Subscribe Model

### Implementation Details

The application implements a **Publish-Subscribe (Pub-Sub) pattern** backed by MQTT topics:

#### Publishers:
- **WebSocket Gateway** publishes validated messages to `MQTT_CHAT_TOPIC` via `pkg/mqttclient`.
- Additional services can publish system events (typing, notifications) to dedicated topics.

#### Subscribers:
- **Gateway fan-out loop** subscribes to deliver events to all live WebSocket clients.
- **Persistence Worker** subscribes to the same topic to write messages into MongoDB asynchronously (`cmd/persistence-worker/main.go`).
- Additional workers (notifications, analytics) can join by subscribing to the topic without touching gateway code.

#### Message Flow:
```
Client A (WebSocket) 
    ↓ [Send JSON payload]
Gateway (web_socket/message.go) 
    ↓ [Publish to MQTT topic]
MQTT Broker (Mosquitto/managed) 
    ↓ [Fan-out to subscribers]
Gateway MQTT consumer   Persistence Worker   Notification Worker (future)
    ↓                          ↓                          ↓
WebSocket clients          MongoDB insert(s)         External channels
```

### Code Implementation:

**Publisher (Gateway WebSocket Handler):**
```go
// web_socket/message.go:123-145
for {
    var msg models.MessagePayload
    if err := c.ReadJSON(&msg); err != nil {
        break
    }
    msg.Date = time.Now()
    mqttClient.PublishJSON(mqttTopic, msg)
}
```

**Subscriber Manager (Gateway MQTT Consumer):**
```go
// web_socket/message.go:60-90
mqttClient.Subscribe(mqttTopic, func(_ mqtt.Client, m mqtt.Message) {
    var payload models.MessagePayload
    json.Unmarshal(m.Payload(), &payload)
    broadcastToClients(payload)
})
```

**Subscriber (Persistence Worker):**
```go
// cmd/persistence-worker/main.go:33-73
mqttClient.Subscribe("", func(_ mqtt.Client, m mqtt.Message) {
    var payload models.MessagePayload
    json.Unmarshal(m.Payload(), &payload)
    repository.InsertNewMessage(saveCtx, payload)
})
```

---

## 3. Multi-Threading for Concurrent Connections

### Implementation

The application uses **Go's goroutines** (lightweight threads) to handle multiple concurrent user connections and message processing.

#### Threading Strategy:

1. **Per-Connection Goroutine**
   - Each WebSocket connection runs in its own goroutine
   - Handled automatically by `websocket.New()` handler
   - Location: `web_socket/message.go:27`

2. **MQTT Consumer Goroutine**
   - MQTT client invokes the subscribed handler on a separate goroutine, which fans out to connected clients.
   - Location: `web_socket/message.go:60-90`

3. **Main Application Goroutine**
   - HTTP server runs in main goroutine
   - WebSocket server runs in separate goroutine
   - Location: `bootstrap/bootstrap.go:37`

### Concurrent Processing:

```
Main Thread
├── HTTP Server (Port 8080)
└── WebSocket Server (Port 9000) [Separate Goroutine]
    ├── Connection Handler 1 [Goroutine]
    ├── Connection Handler 2 [Goroutine]
    ├── Connection Handler N [Goroutine]
    └── MQTT Consumer Handler [Goroutine]
```

### Thread Safety:

- **Client Map**: `map[*websocket.Conn]bool` - accessed by multiple goroutines
- **MQTT Client**: Fan-out handler copies client list under mutex to avoid concurrent map writes
- **Synchronization**: Mutex guards WebSocket client registry

---

## 4. Concurrent Message Handling

### Implementation

The system handles multiple messages concurrently through:

1. **External Queueing via MQTT Topics**
   - Messages are queued in the broker; producers and consumers work independently.
   - MQTT QoS 1 ensures at-least-once delivery even if a consumer is briefly offline.

2. **Concurrent Client Management**
   - Each client connection is independent
   - Messages from different clients are processed in parallel

3. **Broadcast Distribution**
   - MQTT handler iterates over the current client snapshot and writes concurrently without blocking producers.
   - Workers consume in parallel to the gateway so persistence does not block the user experience.

### Performance Characteristics:

- **Throughput**: Can handle multiple messages per second
- **Latency**: Low latency due to event-driven architecture
- **Scalability**: Can handle hundreds of concurrent connections

---

## 5. User Notifications

### Implementation

Notifications are now handled both on the backend and frontend:

1. **Notification Worker (`cmd/notification-worker/main.go`)**
   - Subscribes to MQTT topic `chat/messages`
   - Triggers `notify()` hook which appends entries to `logs/notifications.log` and logs to stdout
   - Demonstrates how a downstream email/SMS/push service can be added without modifying the gateway

```go
mqttClient.Subscribe("", func(_ mqtt.Client, message mqtt.Message) {
    var payload models.MessagePayload
    json.Unmarshal(message.Payload(), &payload)
    notify(payload)
})
```

2. **Browser Notifications (`views/index.html:464-474`)**
   - When each WebSocket client receives a new message, the frontend may call the Web Notifications API to alert the user.

---

## 6. Presence & Cache Layer

- Redis-based presence store (`pkg/presence/store.go`) keeps `presence:<user>` keys with TTL so any gateway replica can check where a user is connected.
- WebSocket gateway marks users online when it receives a valid payload and removes the key when the socket closes (`web_socket/message.go:35-150`).
- Query service exposes `GET /presence/:user` to inspect this shared state.

```go
store, _ := presence.NewStore(env.GetEnv("REDIS_URL", ""), env.GetEnv("GATEWAY_INSTANCE_ID", ""), 45*time.Second)
store.MarkOnline(msg.From)
```

---

## 7. Distributed Services & Containers

Every binary under `cmd/` runs independently so the system keeps working even if one component fails:

| Service / Binary | Responsibility | Interfaces |
|------------------|----------------|------------|
| `chat-frontend` (nginx) | Serves static HTML/JS client from `views/` so UI can be scaled independently | Port `8082` HTTP |
| `message-app` | HTTP API + WebSocket gateway, MQTT publisher/subscriber, Redis presence writer | Ports `3000`, `9000`; MQTT; Redis |
| `persistence-worker` | Subscribes to `MQTT_CHAT_TOPIC` and writes to MongoDB | MQTT ↔ MongoDB |
| `notification-worker` | Subscribes to MQTT and triggers notification hooks / logs | MQTT |
| `query-service` | REST API for `/messages` (Mongo) and `/presence/:user` (Redis) | Port `3100` HTTP |

Supporting containers provided by `docker-compose.yml`: Mosquitto broker (`mqtt-broker`), Redis (`redis`), plus DBs defined in `.env`.

---

## 8. Frontend Delivery

- Static web client is packaged separately using `frontend/Dockerfile` (nginx) and exposed through `chat-frontend` container (`http://localhost:8080`).
- Client JavaScript detects whether it is running behind nginx or gateway and calls the gateway API/WebSocket endpoints accordingly.
- This separation lets the UI scale via CDN or container replicas without impacting the gateway.

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                      │
│  (Browser 1)  (Browser 2)  (Browser 3)  ...  (Browser N)   │
└────────────┬───────────────────┬────────────────────────────┘
             │                   │
             │  WebSocket        │  WebSocket
             │  Connections      │  Connections
             │                   │
┌────────────▼───────────────────▼────────────────────────────┐
│              WebSocket Server (Port 9000)                    │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Connection Handlers (Goroutines)                    │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │   │
│  │  │ Client 1 │  │ Client 2 │  │ Client N │  ...     │   │
│  │  └────┬─────┘  └────┬─────┘  └────┬─────┘          │   │
│  │       │             │             │                  │   │
│  │       └─────────────┼─────────────┘                  │   │
│  │                     │                                │   │
│  │                     ▼                                │   │
│  │            ┌─────────────────┐                      │   │
│  │            │ Broadcast       │                      │   │
│  │            │ Channel         │                      │   │
│  │            │ (Event Bus)     │                      │   │
│  │            └────────┬────────┘                      │   │
│  │                     │                                │   │
│  │                     ▼                                │   │
│  │            ┌─────────────────┐                      │   │
│  │            │ Broadcast       │                      │   │
│  │            │ Manager         │                      │   │
│  │            │ (Goroutine)     │                      │   │
│  │            └────────┬────────┘                      │   │
│  │                     │                                │   │
│  │       ┌─────────────┼─────────────┐                 │   │
│  │       │             │             │                  │   │
│  │       ▼             ▼             ▼                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │   │
│  │  │ Client 1 │  │ Client 2 │  │ Client N │  ...     │   │
│  │  └──────────┘  └──────────┘  └──────────┘          │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
             │
             │ Database Operations
             ▼
┌─────────────────────────────────────────────────────────────┐
│                    MongoDB Database                          │
│              (Message Persistence)                           │
└─────────────────────────────────────────────────────────────┘
```

---

## Key Design Patterns

### 1. Event-Driven Pattern
- Messages flow as events through channels
- Decouples message producers from consumers
- Enables asynchronous processing

### 2. Publish-Subscribe Pattern
- Central event bus (broadcast channel)
- Multiple subscribers (all connected clients)
- Automatic message distribution

### 3. Concurrent Processing Pattern
- Goroutines for parallel connection handling
---

---

## Technology Stack

- **Backend**: Go (Golang)
- **Web Framework**: Fiber
- **WebSocket**: gofiber/contrib/websocket
- **Database**: MongoDB
- **Logging**: ELK Stack (Elasticsearch, Logstash, Kibana)
- **Frontend**: HTML, JavaScript, WebSocket API

---

## Performance Metrics

- **Concurrent Connections**: Supports hundreds of simultaneous connections
- **Message Throughput**: High throughput via event-driven architecture
- **Latency**: Low latency due to direct WebSocket communication
- **Scalability**: Horizontal scaling possible with message queue integration

---

## Future Enhancements

1. **Room/Channel Support**: Multiple chat rooms with topic-based subscriptions (MQTT supports this via wildcards).
2. **Advanced Notifications**: Integrate real SMS/email push providers in the notification worker.
3. **Presence Analytics**: Dedicated analytics worker consuming MQTT to report metrics.
4. **Stronger Auth**: JWT-based WebSocket authentication for gateway + query service.
5. **Multi-region Broker**: Run clustered MQTT (EMQX/Kafka) with replication for HA.

---

## Conclusion

This Real-Time Chat Application successfully implements:
- ✅ **Event-Driven Architecture** with an external MQTT event bus
- ✅ **Publish-Subscribe Model** for message distribution
- ✅ **Multi-Threading** using Go goroutines for concurrent connections
- ✅ **Concurrent Message Handling** with non-blocking operations
- ✅ **User Notifications** via Web Notifications API

The architecture is scalable, maintainable, and follows distributed systems best practices.
