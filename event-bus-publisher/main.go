package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/segmentio/kafka-go"
)

var (
	kafkaTopic      = "ingress-topic"
	kafkaServerHost = "zookeeper-service.kafka-ca1"
	kafkaServerPort = "2181"

	appPort = "8091"
	appName = "EventBusPublisher"
)

func producerHandler(kafkaWriter *kafka.Writer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer r.Body.Close()

		msg := kafka.Message{
			Key:   []byte(fmt.Sprintf("address-%s", r.RemoteAddr)),
			Value: body,
		}
		err = kafkaWriter.WriteMessages(r.Context(), msg)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println(err)
			return
		}

		fmt.Fprintln(w, "message published") // nolint: errcheck
	}
}

func getKafkaWriter(topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(net.JoinHostPort(kafkaServerHost, kafkaServerPort)),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}

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

	kafkaWriter := getKafkaWriter(kafkaTopic)
	defer kafkaWriter.Close()

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "[%s] pong\n", appName)
	})
	http.HandleFunc("/publish", producerHandler(kafkaWriter))

	fmt.Printf("start producer-api at %s ... !!\n", appPort)
	log.Fatal(http.ListenAndServe(":"+appPort, nil))
}
