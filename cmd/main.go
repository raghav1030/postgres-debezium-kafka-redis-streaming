package main

import "github.com/raghav1030/postgres-debezium-kafka-redis-streaming/cmd/kafka_service"

func main() {

	for {

		err := kafka_service.StartConsumerService()
		if err != nil {
			continue
		}
	}
}
