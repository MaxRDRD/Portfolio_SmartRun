package worker

import (
	"context"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker interface {
	ProcessMessage(msg amqp.Delivery) error
}

type WorkerPool struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWorkerPool(url string) (*WorkerPool, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("NewWorkerPool: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("NewWorkerPool: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		conn:   conn,
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}, nil
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

	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()
		for {
			select {
			case <-wp.ctx.Done():
				log.Printf("Consumer context cancelled for queue %s, stopping loop", queueName)
				return
			case d, ok := <-msgs:
				if !ok {
					log.Printf("Messages channel closed for queue %s", queueName)
					return
				}
				err := worker.ProcessMessage(d)
				if err != nil {
					log.Printf("Worker for queue %s failed: %v", queueName, err)
					_ = d.Nack(false, false) // discard message on error for now
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	log.Printf("Started consumer for queue %s", queueName)
	return nil
}

func (wp *WorkerPool) Close() {
	if wp.cancel != nil {
		wp.cancel()
	}
	if wp.ch != nil {
		_ = wp.ch.Close()
	}
	wp.wg.Wait()
	if wp.conn != nil {
		_ = wp.conn.Close()
	}
}
