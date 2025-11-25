package main

import (
	"context"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/pkg/env"
	"github.com/cclli4/realtime-message-app/pkg/mqttclient"
)

func main() {
	env.SetupEnvFile()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	clientID := env.GetEnv("MQTT_NOTIFICATION_CLIENT_ID", fmt.Sprintf("chat-notifier-%d", time.Now().UnixNano()))
	mqttClient, err := mqttclient.NewClient(mqttclient.Config{
		BrokerURL:    env.GetEnv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		ClientID:     clientID,
		Username:     env.GetEnv("MQTT_USERNAME", ""),
		Password:     env.GetEnv("MQTT_PASSWORD", ""),
		DefaultTopic: env.GetEnv("MQTT_CHAT_TOPIC", "chat/messages"),
		QoS:          1,
	})
	if err != nil {
		log.Fatalf("failed to start notification worker: %v", err)
	}
	defer mqttClient.Close()

	log.Printf("Notification worker %s waiting for events...", clientID)
	if err := mqttClient.Subscribe("", func(_ mqtt.Client, message mqtt.Message) {
		var payload models.MessagePayload
		if err := json.Unmarshal(message.Payload(), &payload); err != nil {
			log.Printf("notification worker failed to decode payload: %v", err)
			return
		}

		notify(payload)
	}); err != nil {
		log.Fatalf("notification worker subscription failed: %v", err)
	}

	<-ctx.Done()
	log.Println("notification worker shutting down")
}

func notify(msg models.MessagePayload) {
	if msg.From == "" || msg.Message == "" {
		return
	}
	entry := fmt.Sprintf("NOTIFY %s -> %s\n", msg.From, msg.Message)
	log.Print(entry)

	// Example hook: append to a local file to emulate dispatch queue.
	f, err := os.OpenFile("./logs/notifications.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("notification worker failed to open log file: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(time.Now().Format(time.RFC3339) + " " + entry); err != nil {
		log.Printf("notification worker failed to write log: %v", err)
	}
}
