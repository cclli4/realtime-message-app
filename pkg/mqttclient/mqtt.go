package mqttclient

import (
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"time"
)

// Config describes how to connect to the MQTT broker.
type Config struct {
	BrokerURL    string
	ClientID     string
	Username     string
	Password     string
	DefaultTopic string
	QoS          byte
}

// Client wraps the paho MQTT client with helpers for JSON publishing/subscribing.
type Client struct {
	client       mqtt.Client
	defaultTopic string
	qos          byte
}

// NewClient dials the broker using the provided configuration.
func NewClient(cfg Config) (*Client, error) {
	if cfg.BrokerURL == "" {
		return nil, fmt.Errorf("broker URL is required")
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.BrokerURL)
	opts.SetClientID(cfg.ClientID)
	opts.SetCleanSession(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second)
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Printf("[MQTT] connection lost: %v", err)
	}
	opts.OnConnect = func(_ mqtt.Client) {
		log.Printf("[MQTT] connected to %s as %s", cfg.BrokerURL, cfg.ClientID)
	}

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	qos := cfg.QoS
	if qos > 2 {
		qos = 1
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if ok := token.WaitTimeout(10 * time.Second); !ok {
		return nil, fmt.Errorf("timeout connecting to MQTT broker")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %w", err)
	}

	return &Client{
		client:       client,
		defaultTopic: cfg.DefaultTopic,
		qos:          qos,
	}, nil
}

// PublishJSON marshals payload as JSON and publishes to the supplied topic (or default topic if empty).
func (c *Client) PublishJSON(topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal MQTT payload: %w", err)
	}
	return c.Publish(topic, data)
}

// Publish sends raw bytes to the broker.
func (c *Client) Publish(topic string, payload []byte) error {
	if topic == "" {
		topic = c.defaultTopic
	}
	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	token := c.client.Publish(topic, c.qos, false, payload)
	if ok := token.WaitTimeout(5 * time.Second); !ok {
		return fmt.Errorf("timeout publishing MQTT message")
	}
	return token.Error()
}

// Subscribe registers an async handler for a topic.
func (c *Client) Subscribe(topic string, handler mqtt.MessageHandler) error {
	if topic == "" {
		topic = c.defaultTopic
	}
	if topic == "" {
		return fmt.Errorf("topic is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	token := c.client.Subscribe(topic, c.qos, handler)
	if ok := token.WaitTimeout(5 * time.Second); !ok {
		return fmt.Errorf("timeout subscribing to MQTT topic")
	}
	return token.Error()
}

// Close disconnects gracefully from the broker.
func (c *Client) Close() {
	if c == nil || c.client == nil {
		return
	}
	c.client.Disconnect(250)
}
