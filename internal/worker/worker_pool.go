package worker

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker interface {
	ProcessMessage(msg amqp.Delivery) error
}

type WorkerPool struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewWorkerPool(url string) (*WorkerPool, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("NewWorkerPool: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("NewWorkerPool: %w", err)
	}
	return &WorkerPool{conn: conn, ch: ch}, nil
}

func (wp *WorkerPool) StartConsumer(queueName string, worker Worker) error {
	msgs, err := wp.ch.Consume(
		queueName,
		"",    // consumer
		false, // auto-ack (we will ack manually)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("StartConsumer: %w", err)
	}

	go func() {
		for d := range msgs {
			err := worker.ProcessMessage(d)
			if err != nil {
				log.Printf("Worker for queue %s failed: %v", queueName, err)
				d.Nack(false, false) // discard message on error for now
			} else {
				d.Ack(false)
			}
		}
	}()

	log.Printf("Started consumer for queue %s", queueName)
	return nil
}

func (wp *WorkerPool) Close() {
	if wp.ch != nil {
		_ = wp.ch.Close()
	}
	if wp.conn != nil {
		_ = wp.conn.Close()
	}
}
