package kafka_service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/linkedin/goavro/v2"
)

func StartConsumerService() error {
	topic1 := "comments"
	topic2 := "postgres.public.student"

	workerURL := []string{"kafka:9092"}
	worker, err := connectWorker(workerURL)
	if err != nil {
		return err
	}

	consumer1, err := worker.ConsumePartition(topic1, 0, sarama.OffsetOldest)
	if err != nil {
		return err
	}

	consumer2, err := worker.ConsumePartition(topic2, 0, sarama.OffsetOldest)
	if err != nil {
		return err
	}

	fmt.Println("Consumer started")

	signchan := make(chan os.Signal, 1)
	signal.Notify(signchan, syscall.SIGINT, syscall.SIGTERM)

	doneChan := make(chan struct{})

	commentMessageCount := 0
	testMessageCount := 0

	studentValueSchema, err := getSchema("http://schema-registry:8081", "postgres.public.student-value")
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}

	codec, err := goavro.NewCodec(studentValueSchema)
	if err != nil {
		fmt.Println("Error creating Avro codec:", err)
		return err
	}

	go func() {
		for {
			select {
			case err := <-consumer1.Errors():
				fmt.Println("Consumer1 error:", err)

			case err := <-consumer2.Errors():
				fmt.Println("Consumer2 error:", err)

			case cmtMsg := <-consumer1.Messages():
				commentMessageCount++
				fmt.Printf("Received comment message Count: %d | Topic: %s | Message: %s\n", commentMessageCount, string(cmtMsg.Topic), string(cmtMsg.Value))

			case testMsg := <-consumer2.Messages():
				testMessageCount++
				fmt.Println("Received Avro message type:", reflect.TypeOf(testMsg.Value))

				// Decode Avro-encoded message
				native, _, err := codec.NativeFromBinary(testMsg.Value)
				if err != nil {
					fmt.Println("Error decoding Avro message:", err)
					continue
				}

				// Convert native Go data to JSON
				jsonData, err := json.Marshal(native)
				if err != nil {
					fmt.Println("Error marshalling to JSON:", err)
					continue
				}

				fmt.Printf("Received student message Count: %d | Topic: %s | Message: %s\n", testMessageCount, string(testMsg.Topic), string(jsonData))

			case <-signchan:
				fmt.Println("Interruption detected")
				doneChan <- struct{}{}
			}
		}
	}()

	<-doneChan
	if err = worker.Close(); err != nil {
		return err
	}

	return nil
}

func getSchema(schemaRegistryURL, subject string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/subjects/%s/versions/latest", schemaRegistryURL, subject))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching schema: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println("Schema response:", string(body)) // Debug line

	return string(body), nil
}

func connectWorker(workerURL []string) (sarama.Consumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true

	conn, err := sarama.NewConsumer(workerURL, config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return conn, nil
}
