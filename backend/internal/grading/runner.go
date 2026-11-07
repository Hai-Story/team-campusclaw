package grading

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func event(module, name string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	fields["level"] = "info"
	fields["module"] = module
	fields["event"] = name
	data, _ := json.Marshal(fields)
	fmt.Fprintln(os.Stdout, string(data))
}
func retryDelay(base time.Duration, attempt int) time.Duration {
	delay := base
	for i := 1; i < attempt && delay < 30*time.Second; i++ {
		delay *= 2
	}
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	var bytes [8]byte
	rand.Read(bytes[:])
	jitter := time.Duration(binary.LittleEndian.Uint64(bytes[:]) % uint64(delay/5+1))
	return delay + jitter
}

func RunRelay(ctx context.Context, store *Store, c Settings) {
	for ctx.Err() == nil {
		broker, err := OpenBroker(c.AMQPURL)
		if err != nil {
			event("relay", "broker_unavailable", nil)
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}
		for ctx.Err() == nil {
			item, err := store.ClaimEvent(ctx)
			if err != nil {
				event("relay", "storage_unavailable", nil)
				if !wait(ctx, time.Second) {
					break
				}
				continue
			}
			if item == nil {
				if !wait(ctx, c.Poll) {
					break
				}
				continue
			}
			publish, cancel := context.WithTimeout(ctx, 5*time.Second)
			err = broker.Send(publish, Message{item.TaskID, item.Kind, item.Generation})
			cancel()
			markErr := store.Published(ctx, *item, err == nil)
			if err != nil {
				event("relay", "publish_retry", map[string]any{"task_id": item.TaskID})
				break
			}
			if markErr != nil {
				event("relay", "confirm_record_pending", map[string]any{"task_id": item.TaskID})
				break
			}
			event("relay", "published", map[string]any{"task_id": item.TaskID, "kind": item.Kind, "publish_attempts": item.Attempts})
		}
		broker.Close()
		if !wait(ctx, c.Poll) {
			return
		}
	}
}

func RunWorker(ctx context.Context, store *Store, c Settings) {
	for ctx.Err() == nil {
		broker, err := OpenBroker(c.AMQPURL)
		if err != nil {
			event("worker", "broker_unavailable", nil)
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}
		messages, err := broker.consumer.Consume(WorkQueue, "", false, false, false, false, nil)
		if err != nil {
			broker.Close()
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}
		working := true
		for working && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				working = false
			case delivery, open := <-messages:
				if !open {
					working = false
					break
				}
				var message Message
				if json.Unmarshal(delivery.Body, &message) != nil || !taskPattern.MatchString(message.TaskID) || message.Kind != "work" {
					delivery.Reject(false)
					event("worker", "invalid_message", nil)
					continue
				}
				for ctx.Err() == nil {
					work, err := store.ClaimTask(ctx, message.TaskID, c)
					if delay, busy := IsBusy(err); busy {
						if delay > 500*time.Millisecond {
							delay = 500 * time.Millisecond
						}
						if !wait(ctx, delay) {
							working = false
							break
						}
						continue
					}
					if err != nil {
						delivery.Nack(false, true)
						wait(ctx, 500*time.Millisecond)
						break
					}
					if work == nil {
						delivery.Ack(false)
						break
					}
					start := time.Now()
					event("grading", "started", map[string]any{"task_id": work.TaskID, "actor_id": work.ActorID, "skill_id": VerdictVersion, "status": "running", "attempts": work.Attempts})
					call, cancel := context.WithTimeout(ctx, c.Timeout)
					feedback, usage, failure := Feedback(call, c, *work)
					cancel()
					status := "succeeded"
					if failure != nil {
						err = store.Fail(ctx, *work, failure, c, retryDelay(c.RetryBase, work.Attempts))
						status = "pending"
						if !failure.Retry || work.Attempts >= c.MaxAttempts {
							status = "failed"
						}
					} else {
						var applied bool
						applied, err = store.Complete(ctx, *work, feedback, usage)
						if !applied && err == nil {
							status = "discarded"
						}
					}
					if err != nil {
						delivery.Nack(false, true)
						event("grading", "storage_unavailable", map[string]any{"task_id": work.TaskID})
						break
					}
					event("grading", "completed", map[string]any{"task_id": work.TaskID, "actor_id": work.ActorID, "skill_id": VerdictVersion, "status": status, "attempts": work.Attempts, "duration_ms": time.Since(start).Milliseconds(), "usage": usage, "model": c.Model})
					// This ACK follows the result/state transaction, including durable retry/dead events.
					delivery.Ack(false)
					break
				}
			}
		}
		broker.Close()
		if !wait(ctx, 200*time.Millisecond) {
			return
		}
	}
}
