package main

import (
	"context"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/app/repository"
	"github.com/cclli4/realtime-message-app/pkg/database"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"github.com/cclli4/realtime-message-app/pkg/mqttclient"
)

func main() {
	env.SetupEnvFile()
	database.SetupMongoDB()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	workerClientID := env.GetEnv("MQTT_WORKER_CLIENT_ID", fmt.Sprintf("chat-worker-%d", time.Now().UnixNano()))
	mqttClient, err := mqttclient.NewClient(mqttclient.Config{
		BrokerURL:    env.GetEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ClientID:     workerClientID,
		Username:     env.GetEnv("MQTT_USERNAME", ""),
		Password:     env.GetEnv("MQTT_PASSWORD", ""),
		DefaultTopic: env.GetEnv("MQTT_CHAT_TOPIC", "chat/messages"),
		QoS:          1,
	})
	if err != nil {
		log.Fatalf("failed to start persistence worker MQTT client: %v", err)
	}
	defer mqttClient.Close()

	log.Printf("Persistence worker %s waiting for events...", workerClientID)

	if err := mqttClient.Subscribe("", func(_ mqtt.Client, message mqtt.Message) {
		var payload models.MessagePayload
		if err := json.Unmarshal(message.Payload(), &payload); err != nil {
			log.Printf("failed to decode MQTT payload: %v", err)
			return
		}

		if payload.Date.IsZero() {
			payload.Date = time.Now()
		}

		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repository.InsertNewMessage(saveCtx, payload); err != nil {
			log.Printf("failed to persist message from %s: %v", payload.From, err)
		} else {
			log.Printf("persisted message from %s at %s", payload.From, payload.Date.Format(time.RFC3339))
		}
	}); err != nil {
		log.Fatalf("failed to subscribe persistence worker: %v", err)
	}

	<-ctx.Done()
	log.Println("persistence worker received shutdown signal")
}
