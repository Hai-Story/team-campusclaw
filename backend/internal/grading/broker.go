package grading

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const WorkQueue = "grade.jobs"
const DeadQueue = "grade.dead"

type Message struct {
	TaskID     string `json:"task_id"`
	Kind       string `json:"kind"`
	Generation int    `json:"generation"`
}
type Broker struct {
	connection          *amqp.Connection
	publisher, consumer *amqp.Channel
	returns             chan amqp.Return
}

func OpenBroker(url string) (*Broker, error) {
	connection, err := amqp.DialConfig(url, amqp.Config{Heartbeat: 10 * time.Second, Locale: "en_US", Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return nil, err
	}
	b := &Broker{connection: connection}
	ok := false
	defer func() {
		if !ok {
			b.Close()
		}
	}()
	b.publisher, err = connection.Channel()
	if err != nil {
		return nil, err
	}
	for _, name := range []string{WorkQueue, DeadQueue} {
		if _, err = b.publisher.QueueDeclare(name, true, false, false, false, nil); err != nil {
			return nil, err
		}
	}
	if err = b.publisher.Confirm(false); err != nil {
		return nil, err
	}
	b.returns = b.publisher.NotifyReturn(make(chan amqp.Return, 1))
	b.consumer, err = connection.Channel()
	if err != nil {
		return nil, err
	}
	if err = b.consumer.Qos(1, 0, false); err != nil {
		return nil, err
	}
	ok = true
	return b, nil
}
func (b *Broker) Close() {
	if b.connection != nil {
		b.connection.Close()
	}
}
func (b *Broker) Send(ctx context.Context, message Message) error {
	result := make(chan error, 1)
	go func() { result <- b.sendConfirmed(ctx, message) }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		b.connection.CloseDeadline(time.Now().Add(100 * time.Millisecond))
		return ctx.Err()
	}
}
func (b *Broker) sendConfirmed(ctx context.Context, message Message) error {
	queue := WorkQueue
	if message.Kind == "dead" {
		queue = DeadQueue
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	confirmation, err := b.publisher.PublishWithDeferredConfirmWithContext(ctx, "", queue, true, false, amqp.Publishing{DeliveryMode: amqp.Persistent, ContentType: "application/json", MessageId: message.TaskID, Timestamp: time.Now().UTC(), Body: body})
	if err != nil {
		return err
	}
	if confirmation == nil {
		return errors.New("publisher confirm unavailable")
	}
	ack, err := confirmation.WaitContext(ctx)
	if err != nil || !ack {
		return errors.New("publish not confirmed")
	}
	select {
	case <-b.returns:
		return errors.New("message unroutable")
	default:
		return nil
	}
}
