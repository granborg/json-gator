package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// QoS represents MQTT Quality of Service levels
type QoS int

const (
	AtMostOnce  QoS = 0 // Fire and forget
	AtLeastOnce QoS = 1 // Guaranteed delivery but may be delivered more than once
	ExactlyOnce QoS = 2 // Guaranteed delivery exactly once
)

// PublishType represents MQTT publish/subscribe modes
type PublishType int

const (
	Pub    PublishType = 0 // Publish only
	Sub    PublishType = 1 // Subscribe only
	PubSub PublishType = 2 // Both publish and subscribe
)

// MqttPath represents an MQTT topic configuration
type MqttPath struct {
	Topic       string      `json:"topic"`
	Qos         QoS         `json:"qos"`
	Retain      bool        `json:"retain"`
	PublishType PublishType `json:"publishType"`
}

// MqttClient represents an MQTT client configuration
type MqttClient struct {
	Broker           string                `json:"broker"`
	Username         string                `json:"username"`
	Password         string                `json:"password"`
	Secure           bool                  `json:"secure"`
	CaCert           string                `json:"caCert"`
	ClientCert       string                `json:"clientCert"`
	ClientKey        string                `json:"clientKey"`
	CaServerHostname string                `json:"caServerHostname"`
	Paths            map[string][]MqttPath `json:"paths"`
	Client           mqtt.Client
	callbacks        map[string]mqtt.MessageHandler
}

func (m *MqttClient) Connect() {
	// Create MQTT client options
	opts := mqtt.NewClientOptions()

	// Use TLS/SSL broker endpoint
	opts.AddBroker(m.Broker)
	opts.SetClientID("gator-" + fmt.Sprintf("%d", time.Now().Unix()))
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(messageHandler)
	opts.SetPingTimeout(1 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Second)

	// Optional: Add username and password authentication
	if m.Username != "" || m.Password != "" {
		opts.SetUsername("your-username")
		opts.SetPassword("your-password")
	}

	if m.Secure {
		// Set up TLS configuration with certificates
		tlsConfig, err := newTLSConfig(
			m.CaCert,           // CA certificate
			m.ClientCert,       // Client certificate
			m.ClientKey,        // Client private key
			m.CaServerHostname, // Empty server hostname skips hostname verification
		)
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}

		// Set TLS config to client options
		opts.SetTLSConfig(tlsConfig)
	}

	// Set up connection callback handlers
	opts.SetOnConnectHandler(connectHandler)
	opts.SetConnectionLostHandler(connectionLostHandler)

	// Create the client
	m.Client = mqtt.NewClient(opts)

	// Connect to the broker
	if token := m.Client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Error connecting to MQTT broker: %v", token.Error())
	}
}

func (m *MqttClient) SetupSubscriptions(setModelDataCallback func([]string, any, bool) error) {
	// Initialize the callbacks map if it doesn't exist
	if m.callbacks == nil {
		m.callbacks = make(map[string]mqtt.MessageHandler)
	}

	for path, val := range m.Paths {
		for _, mqttPath := range val {
			if mqttPath.PublishType != Sub && mqttPath.PublishType != PubSub {
				continue // Skip topics we're not subscribing to
			}

			topic := mqttPath.Topic
			pathTokens := strings.Split(path, "/")

			// Create a unique key for this callback
			callbackKey := fmt.Sprintf("%s-%s", topic, path)

			// Store the callback in the map to prevent it from being garbage collected
			m.callbacks[callbackKey] = func(client mqtt.Client, msg mqtt.Message) {
				log.Printf("Received message on topic: %s with payload: %s", msg.Topic(), string(msg.Payload()))

				// Create a local copy of pathTokens to ensure it's not modified
				localPathTokens := make([]string, len(pathTokens))
				copy(localPathTokens, pathTokens)

				// Unmarshal the message payload
				var data any
				err := json.Unmarshal(msg.Payload(), &data)
				if err != nil {
					log.Printf("Error unmarshaling message from topic %s: %v", msg.Topic(), err)
					// Try to use the raw payload as a string if JSON unmarshaling fails
					data = string(msg.Payload())
				}

				// Call the setModelDataCallback with path segments and unmarshaled data
				err = setModelDataCallback(localPathTokens, data, true)
				if err != nil {
					log.Printf("Error in setModelDataCallback for topic %s: %v", msg.Topic(), err)
				} else {
					log.Printf("Successfully processed message for path \"%s\"", strings.Join(localPathTokens, "/"))
				}
			}

			// Use the stored callback for subscription
			token := m.Client.Subscribe(topic, byte(mqttPath.Qos), m.callbacks[callbackKey])
			token.Wait()

			if token.Error() != nil {
				log.Printf("Error subscribing to topic %s: %v", topic, token.Error())
				// Don't disconnect immediately, continue with other subscriptions
				continue
			}
			log.Printf("Path \"%s\" is subscribed to topic: \"%s\"", path, topic)
		}
	}
}

func (m *MqttClient) Disconnect() {
	m.Client.Disconnect(250)
	log.Println("MQTT client disconnected")
}

// Publish a message to a specified JSON path and trigger any associated publishes
func (m *MqttClient) PublishMessage(pathTokens []string, getModelDataCallback func([]string, bool) (any, error)) error {
	fullPath := strings.Join(pathTokens, "/")
	var err error
	var tokens []mqtt.Token

	// Publish to all matching mqtt paths
	for path, mqttPaths := range m.Paths {
		if !strings.HasPrefix(fullPath, path) {
			continue
		}

		// We need to get the payload for this path, even if it was triggered by a field deeper within the object.
		payloadObj, err := getModelDataCallback(strings.Split(path, "/"), false)
		if err != nil {
			return err
		}

		payload, err := json.Marshal(payloadObj)
		if err != nil {
			return err
		}

		for _, mqttPath := range mqttPaths {
			if mqttPath.PublishType == Pub || mqttPath.PublishType == PubSub {
				token := m.Client.Publish(mqttPath.Topic, byte(mqttPath.Qos), mqttPath.Retain, payload)
				log.Printf("Published payload to topic \"%s\": %s", mqttPath.Topic, string(payload))

				tokens = append(tokens, token)
			}
		}
	}

	// Wait for all publishes to complete
	for _, token := range tokens {
		token.Wait()
		if token.Error() != nil {
			err = fmt.Errorf("Error publishing: %v", token.Error())
		}
	}

	return err
}

// Create a new TLS configuration for secure MQTT connections
func newTLSConfig(caFile, certFile, keyFile, serverName string) (*tls.Config, error) {
	// Load CA certificate
	rootCA, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("error reading CA certificate: %v", err)
	}

	// Create CA certificate pool and add the CA
	rootCAPool := x509.NewCertPool()
	if ok := rootCAPool.AppendCertsFromPEM(rootCA); !ok {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Load client certificate and private key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("error loading client certificate and key: %v", err)
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		RootCAs:            rootCAPool,
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: serverName == "", // Skip verification if no server name provided
		Certificates:       []tls.Certificate{cert},
	}

	// Only set ServerName if one was provided
	if serverName != "" {
		tlsConfig.ServerName = serverName
	}

	return tlsConfig, nil
}

// Default message handler
var messageHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message on topic: %s", msg.Topic())
	log.Printf("Message: %s", msg.Payload())
}

// Connect handler - called when connection is established
var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected to MQTT broker")
}

// Connection lost handler - called when connection is lost
var connectionLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Connection lost: %v", err)
}
