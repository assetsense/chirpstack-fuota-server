package fuota

import (
	"context"
	"log"
	"time"

	"github.com/chirpstack/chirpstack/api/go/v4/integration"
	"github.com/streadway/amqp"
	"google.golang.org/protobuf/proto"
)

var (
	rabbitMQURL  = "amqp://" + C2config.RabbitMQUsername + ":" + C2config.RabbitMQPassword + "@" + C2config.RabbitMQHost + ":" + C2config.RabbitMQPort + "/"
	queueName    = "mgfuota"
	exchangeName = "amq.topic"
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

func (d *Deployment) processMessages(ctx context.Context, msgs <-chan amqp.Delivery) {
	for msg := range msgs {
		go d.processEachMessage(ctx, msg)
	}
}

func (d *Deployment) processEachMessage(ctx context.Context, msg amqp.Delivery) {
	var uplinkEvent integration.UplinkEvent
	if err := proto.Unmarshal(msg.Body, &uplinkEvent); err != nil {
		log.Printf("Error decoding Protobuf message: %v", err)
		return
	}

	// if err := json.Unmarshal(msg.Body, &uplinkEvent); err != nil {
	// 	log.Printf("Error decoding uplink message: %v", err)
	// }

	if err := d.HandleUplinkEvent(ctx, uplinkEvent); err != nil {
		log.Printf("Error handling uplink event: %v", err)
	}
}

func createQueue(ch *amqp.Channel) {
	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")
	// fmt.Println("queue created")

	routingKeys := []string{"200", "201", "202"}
	for _, routingkey := range routingKeys {
		err = ch.QueueBind(
			q.Name,       // queue name
			routingkey,   // routing key
			exchangeName, // exchange
			false,
			nil,
		)
		failOnError(err, "Failed to bind a queue")
	}
	// fmt.Println("queue binded")
}

func (d *Deployment) ReceiveRabbitMq(ctx context.Context) {
	conn, err := connect()
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err := createChannel(conn)
	failOnError(err, "Failed to open a channel")

	createQueue(ch)

	msgs, err := consumeMessages(ch)
	failOnError(err, "Failed to register a consumer")

	go d.processMessages(ctx, msgs)

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")

}
