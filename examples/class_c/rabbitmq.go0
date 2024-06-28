package main

import (
	"fmt"
	"log"
	"time"

	"github.com/chirpstack/chirpstack/api/go/v4/integration"
	"github.com/streadway/amqp"
	"google.golang.org/protobuf/proto"
)

var (
	rabbitMQURL = "amqp://dev:dev123@localhost:5672/"
	queueName   = "default"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func connect() (*amqp.Connection, error) {
	for {
		conn, err := amqp.Dial(rabbitMQURL)
		if err != nil {
			log.Printf("Failed to connect to RabbitMQ: %v. Retrying in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		} else {
			log.Println("Successfully connected to RabbitMQ")
			return conn, nil
		}
	}
}

// Create a channel
func createChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	return ch, nil
}

// Consume messages from the default queue
func consumeMessages(ch *amqp.Channel) (<-chan amqp.Delivery, error) {
	msgs, err := ch.Consume(
		queueName, // queue name; use empty string for the default queue
		"",        // consumer tag
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func processMessages(msgs <-chan amqp.Delivery) {
	for msg := range msgs {
		// log.Printf("Received a message: %s", msg.Body)
		go processEachMessage(msg)

	}
}

func processEachMessage(msg amqp.Delivery) {
	var uplinkEvent integration.UplinkEvent
	if err := proto.Unmarshal(msg.Body, &uplinkEvent); err != nil {
		log.Printf("Error decoding Protobuf message: %v", err)
		return
	}
	fmt.Println("\nval:", uplinkEvent)
}

func main() {
	conn, err := connect()
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := createChannel(conn)
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	msgs, err := consumeMessages(ch)
	failOnError(err, "Failed to register a consumer")

	// forever := make(chan bool)

	go processMessages(msgs)

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	fmt.Scanln()
}
