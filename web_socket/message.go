package web_socket

import (
	"encoding/json"
	"fmt"
	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"github.com/cclli4/realtime-message-app/pkg/mqttclient"
	"github.com/cclli4/realtime-message-app/pkg/presence"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"log"
	"sync"
	"time"
)

// ServeWSMessaging implements an Event-Driven Architecture with Publish-Subscribe pattern
// for real-time messaging. It uses multi-threading (goroutines) to handle concurrent
// user connections and message distribution.
//
// Architecture Components:
// 1. Event-Driven: Messages flow through MQTT topics (external event bus)
// 2. Pub-Sub: Clients publish messages, all subscribers listen from MQTT
// 3. Multi-Threading: Each connection runs in separate goroutine, MQTT dispatch runs concurrently
// 4. Concurrent: Multiple messages processed simultaneously
//
// @Description Send a message via WebSocket (Event-Driven Architecture)
// @Tags        Message
// @Param 		Authorization header string true "Bearer token" default(Bearer <token>)
// @Param       body body models.MessagePayload true "Message Payload"
// @Success 200 {object} models.MessagePayload
// @Failure		500 {object} response.Response
// @Router      /message/v1/send [post]
func ServeWSMessaging() {
	// Thread-safe client registry for managing concurrent connections
	// Each WebSocket connection is handled in a separate goroutine
	var clients = make(map[*websocket.Conn]bool)
	var clientUsers = make(map[*websocket.Conn]string)
	var clientsMutex sync.RWMutex // Mutex for thread-safe access to clients map

	mqttBrokerURL := env.GetEnv("MQTT_BROKER_URL", "tcp://localhost:1883")
	mqttTopic := env.GetEnv("MQTT_CHAT_TOPIC", "chat/messages")
	mqttClientID := env.GetEnv("MQTT_GATEWAY_CLIENT_ID", fmt.Sprintf("gateway-%d", time.Now().UnixNano()))
	mqttUsername := env.GetEnv("MQTT_USERNAME", "")
	mqttPassword := env.GetEnv("MQTT_PASSWORD", "")
	gatewayInstanceID := env.GetEnv("GATEWAY_INSTANCE_ID", fmt.Sprintf("gateway-%d", time.Now().UnixNano()))
	redisURL := env.GetEnv("REDIS_URL", "")
	var presenceStore *presence.Store
	if redisURL != "" {
		store, err := presence.NewStore(redisURL, gatewayInstanceID, 45*time.Second)
		if err != nil {
			log.Printf("Failed to initialize presence store: %v", err)
		} else {
			presenceStore = store
			defer presenceStore.Close()
		}
	} else {
		log.Println("Redis URL not configured; presence tracking disabled")
	}

	mqttClient, err := mqttclient.NewClient(mqttclient.Config{
		BrokerURL:    mqttBrokerURL,
		ClientID:     mqttClientID,
		Username:     mqttUsername,
		Password:     mqttPassword,
		DefaultTopic: mqttTopic,
		QoS:          1,
	})
	if err != nil {
		log.Printf("Failed to initialize MQTT client: %v", err)
		return
	}
	defer mqttClient.Close()

	broadcastToClients := func(msg models.MessagePayload) {
		clientsMutex.RLock()
		clientList := make([]*websocket.Conn, 0, len(clients))
		for client := range clients {
			clientList = append(clientList, client)
		}
		clientsMutex.RUnlock()

		log.Printf("Broadcasting MQTT message from %s to %d clients", msg.From, len(clientList))
		for _, client := range clientList {
			if err := client.WriteJSON(msg); err != nil {
				log.Printf("Failed to write json to client: %v", err)
				clientsMutex.Lock()
				delete(clients, client)
				clientsMutex.Unlock()
				client.Close()
			}
		}
	}

	if err := mqttClient.Subscribe(mqttTopic, func(_ mqtt.Client, message mqtt.Message) {
		var payload models.MessagePayload
		if err := json.Unmarshal(message.Payload(), &payload); err != nil {
			log.Printf("Failed to decode MQTT payload: %v", err)
			return
		}
		broadcastToClients(payload)
	}); err != nil {
		log.Printf("Failed to subscribe to MQTT topic %s: %v", mqttTopic, err)
		return
	}

	// Create a new Fiber app specifically for WebSocket server
	// This is separate from the main HTTP server
	wsApp := fiber.New()

	// Add a helpful message for HTTP requests to WebSocket server
	wsApp.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("This is a WebSocket server. Please access the main application at http://localhost:3000")
	})

	// WebSocket handler: Each connection runs in a separate goroutine (multi-threading)
	// This allows handling multiple concurrent user connections simultaneously
	// Note: WebSocket uses GET with upgrade, not POST
	wsApp.Get("/message/v1/send", websocket.New(func(c *websocket.Conn) {
		// Register client (Subscriber) - thread-safe operation
		clientsMutex.Lock()
		clients[c] = true
		clientCount := len(clients)
		clientsMutex.Unlock()

		log.Printf("New WebSocket client connected. Total clients: %d", clientCount)

		// Cleanup when connection closes
		defer func() {
			clientsMutex.Lock()
			delete(clients, c)
			user := clientUsers[c]
			delete(clientUsers, c)
			remainingClients := len(clients)
			clientsMutex.Unlock()
			c.Close()
			log.Printf("WebSocket client disconnected. Remaining clients: %d", remainingClients)
			if presenceStore != nil && user != "" {
				if err := presenceStore.MarkOffline(user); err != nil {
					log.Printf("Failed to mark %s offline: %v", user, err)
				}
			}
		}()

		// Event Producer: Read messages from client and publish to event bus
		// This loop runs concurrently for each connected client
		for {
			var msg models.MessagePayload
			if err := c.ReadJSON(&msg); err != nil {
				log.Println("error payload: ", err)
				break
			}

			msg.Date = time.Now()

			// Validate message before processing
			if msg.From == "" || msg.Message == "" {
				log.Printf("Invalid message: missing from or message field")
				continue
			}

			if presenceStore != nil {
				clientsMutex.Lock()
				clientUsers[c] = msg.From
				clientsMutex.Unlock()
				if err := presenceStore.MarkOnline(msg.From); err != nil {
					log.Printf("Failed to mark %s online: %v", msg.From, err)
				}
			} else {
				clientsMutex.Lock()
				clientUsers[c] = msg.From
				clientsMutex.Unlock()
			}

			log.Printf("Publishing message from %s to MQTT topic %s", msg.From, mqttTopic)
			if err := mqttClient.PublishJSON(mqttTopic, msg); err != nil {
				log.Printf("Failed to publish message to MQTT: %v", err)
				continue
			}
		}
	}))

	wsHost := env.GetEnv("APP_HOST", "localhost")
	wsPort := env.GetEnv("APP_PORT_SOCKET", "9000")

	// Ensure host and port are not empty
	if wsHost == "" {
		wsHost = "localhost"
	}
	if wsPort == "" {
		wsPort = "9000"
	}

	wsAddress := fmt.Sprintf("%s:%s", wsHost, wsPort)
	log.Printf("WebSocket server starting on %s", wsAddress)
	log.Printf("WebSocket server configuration - Host: %s, Port: %s", wsHost, wsPort)

	// Start WebSocket server (this is blocking, so it will run in the goroutine)
	// Use Listen instead of ListenTLS for WebSocket
	if err := wsApp.Listen(wsAddress); err != nil {
		log.Printf("WebSocket server failed to start: %v", err)
		// Log the error but don't exit - the main HTTP server should continue running
		return
	}
}
