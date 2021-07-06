package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaIngressTopic = "ingress-topic"
	kafkaEgressTopic  = "egress-topic"
	kafkaServerHost   = "zookeeper-service.kafka-ca1"
	kafkaServerPort   = "2181"

	appPort = "8096"
	appName = "EventBusListener"
)

func main() {
	if host, ok := os.LookupEnv("KAFKA_HOST"); ok {
		kafkaServerHost = host
	}
	if port, ok := os.LookupEnv("ZOOKEEPER_PORT"); ok {
		kafkaServerPort = port
	}
	if port, ok := os.LookupEnv("PORT"); ok {
		appPort = port
	}

	reader := setupKafkaReader(kafkaIngressTopic)
	defer reader.Close()

	writer := setupKafkaWriter(kafkaEgressTopic)
	defer writer.Close()

	go receiveMessages(context.Background(), reader, basicHandler(writer))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "[%s] pong\n", appName)
	})

	log.Fatalln(http.ListenAndServe(":"+appPort, nil))
}

func setupKafkaReader(topic string) *kafka.Reader {
	broker := net.JoinHostPort(kafkaServerHost, kafkaServerPort)
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{broker},
		GroupID:        "ingress-topic-listener",
		Topic:          topic,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
	})
}

func setupKafkaWriter(topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(net.JoinHostPort(kafkaServerHost, kafkaServerPort)),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}

type Handler func(ctx context.Context, msg kafka.Message)

func receiveMessages(ctx context.Context, reader *kafka.Reader, handler Handler) {
	for {
		select {
		case <-ctx.Done():
			log.Println("stopping kafka consumer")
			return
		default:
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("[ERROR] %s\n", err)
				continue
			}

			handler(context.Background(), msg)
		}
	}
}

func basicHandler(writer *kafka.Writer) Handler {
	return func(_ context.Context, msg kafka.Message) {
		log.Printf("message at topic:%v partition:%v offset:%v  %s = %s\n",
			msg.Topic,
			msg.Partition,
			msg.Offset,
			string(msg.Key),
			string(msg.Value),
		)

		value := strings.ToUpper(string(msg.Value))

		out := kafka.Message{
			Key:   msg.Key,
			Value: []byte(value),
		}

		if err := writer.WriteMessages(context.Background(), out); err != nil {
			log.Printf("[ERROR] failed to publish messages: %v\n", err)
		}
		log.Println("message published")
	}
}
