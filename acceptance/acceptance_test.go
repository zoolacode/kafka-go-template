// +build acceptance

package acceptance

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: this should be provided as environment variables
const (
	publisherUrl = "http://eventbuspublisherservice.kafka-ca1:8097"
	listenerUrl  = "http://eventbuslistenerservice.kafka-ca1:8096"
)

var (
	kafkaIngressTopic = "ingress-topic"
	kafkaEgressTopic  = "egress-topic"
	kafkaServerHost   = "zookeeper-service.kafka-ca1"
	kafkaServerPort   = "2181"

	ingressWriter *kafka.Writer
	egressReader  *kafka.Reader
)

var messages = make(map[string]kafka.Message)

func TestMain(m *testing.M) {
	// setup steps here
	if host, ok := os.LookupEnv("KAFKA_HOST"); ok {
		kafkaServerHost = host
	}
	if port, ok := os.LookupEnv("ZOOKEEPER_PORT"); ok {
		kafkaServerPort = port
	}

	ingressWriter = &kafka.Writer{
		Addr:     kafka.TCP(net.JoinHostPort(kafkaServerHost, kafkaServerPort)),
		Topic:    kafkaIngressTopic,
		Balancer: &kafka.LeastBytes{},
	}

	broker := net.JoinHostPort(kafkaServerHost, kafkaServerPort)
	egressReader = kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{broker},
		GroupID:        "egress-topic-listener",
		Topic:          kafkaEgressTopic,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
	})

	// wait for kafka resources to spin up
	// TODO: find a better way to deal with long setup time if possible
	time.Sleep(15 * time.Second)

	go receiveEgressMessages()

	code := m.Run()

	// teardown & cleanup actions
	ingressWriter.Close()
	egressReader.Close()

	os.Exit(code)
}

func TestPing(t *testing.T) {
	req, err := http.Get(publisherUrl + "/ping")
	require.NoError(t, err)
	defer req.Body.Close()

	assert.Equal(t, http.StatusOK, req.StatusCode)

	data, err := ioutil.ReadAll(req.Body)
	require.NoError(t, err)

	code := string(data)
	assert.True(t, strings.Contains(code, "pong"), code)
}

func TestMessageToUpper(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{
			key:   "test-message-to-upper-id",
			value: "some very basic and short string",
		},
		{
			key:   "test-message-to-upper-id",
			value: "some other string",
		},
		{
			key:   "test-message-to-upper-id",
			value: "absolutely different bunch of words",
		},
		{
			key:   "test-message-to-upper-id",
			value: "yep, another one",
		},
		{
			key:   "test-message-to-upper-id",
			value: "finally the last one",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("%s-%d", test.key, i), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			key := fmt.Sprintf("%s-%d", test.key, i)

			t.Logf("[%v] publishing message\n", time.Now().Format(time.RFC3339))
			err := ingressWriter.WriteMessages(ctx, kafka.Message{
				Key:   []byte(key),
				Value: []byte(test.value),
			})
			require.NoError(t, err)
			t.Logf("[%v] message published\n", time.Now().Format(time.RFC3339))

			require.Eventually(t,
				func() bool {
					_, ok := messages[key]
					return ok
				},
				30*time.Second,
				100*time.Millisecond,
			)
			t.Logf("[%v] got message\n", time.Now().Format(time.RFC3339))

			assert.Equal(t, strings.ToUpper(test.value), string(messages[key].Value))
			cancel()
		})
	}
}

func receiveEgressMessages() {
	for {
		msg, err := egressReader.ReadMessage(context.Background())
		if err != nil {
			log.Fatalln(err)
		}
		messages[string(msg.Key)] = msg
	}
}
