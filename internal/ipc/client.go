package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

type RawResponse struct {
	ID      string
	Action  Action
	Success bool
	Payload json.RawMessage
	Error   *ErrorPayload
}

type RawEvent struct {
	Event   string
	Payload json.RawMessage
}

type Client struct {
	conn net.Conn
	enc  *json.Encoder

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan RawResponse

	events chan RawEvent
	errs   chan error

	nextID uint64
	closed chan struct{}
	once   sync.Once
}

func DialClient(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to ipc socket %s: %w", socketPath, err)
	}

	client := &Client{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		pending: make(map[string]chan RawResponse),
		events:  make(chan RawEvent, 32),
		errs:    make(chan error, 1),
		closed:  make(chan struct{}),
	}
	go client.readLoop()
	return client, nil
}

func (c *Client) Events() <-chan RawEvent {
	return c.events
}

func (c *Client) Errors() <-chan error {
	return c.errs
}

func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		err = c.conn.Close()

		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
	})
	return err
}

func (c *Client) Request(ctx context.Context, action Action, payload interface{}, out interface{}) error {
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.nextID, 1))

	var encodedPayload json.RawMessage
	if payload != nil {
		bytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request payload: %w", err)
		}
		encodedPayload = bytes
	}

	responseCh := make(chan RawResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = responseCh
	c.pendingMu.Unlock()

	request := Request{
		ID:      id,
		Action:  action,
		Payload: encodedPayload,
	}

	c.writeMu.Lock()
	err := c.enc.Encode(request)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return fmt.Errorf("send ipc request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return ctx.Err()
	case raw, ok := <-responseCh:
		if !ok {
			return errors.New("ipc client closed while waiting for response")
		}
		if !raw.Success {
			if raw.Error == nil {
				return fmt.Errorf("ipc %s failed", action)
			}
			return fmt.Errorf("ipc %s failed: %s", action, raw.Error.Message)
		}
		if out != nil && len(raw.Payload) > 0 {
			if err := json.Unmarshal(raw.Payload, out); err != nil {
				return fmt.Errorf("decode ipc response payload: %w", err)
			}
		}
		return nil
	}
}

func (c *Client) readLoop() {
	defer close(c.events)
	defer close(c.errs)

	decoder := json.NewDecoder(c.conn)
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			select {
			case <-c.closed:
				return
			default:
			}
			select {
			case c.errs <- fmt.Errorf("read ipc message: %w", err):
			default:
			}
			return
		}

		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			select {
			case c.errs <- fmt.Errorf("decode ipc envelope: %w", err):
			default:
			}
			continue
		}

		switch envelope.Type {
		case messageTypeResponse:
			var response struct {
				Type    string          `json:"type"`
				ID      string          `json:"id"`
				Action  Action          `json:"action"`
				Success bool            `json:"success"`
				Payload json.RawMessage `json:"payload,omitempty"`
				Error   *ErrorPayload   `json:"error,omitempty"`
			}
			if err := json.Unmarshal(raw, &response); err != nil {
				select {
				case c.errs <- fmt.Errorf("decode ipc response: %w", err):
				default:
				}
				continue
			}

			c.pendingMu.Lock()
			ch, ok := c.pending[response.ID]
			if ok {
				delete(c.pending, response.ID)
			}
			c.pendingMu.Unlock()
			if ok {
				ch <- RawResponse{
					ID:      response.ID,
					Action:  response.Action,
					Success: response.Success,
					Payload: response.Payload,
					Error:   response.Error,
				}
				close(ch)
			}
		case messageTypeEvent:
			var event struct {
				Type    string          `json:"type"`
				Event   string          `json:"event"`
				Payload json.RawMessage `json:"payload,omitempty"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				select {
				case c.errs <- fmt.Errorf("decode ipc event: %w", err):
				default:
				}
				continue
			}
			select {
			case c.events <- RawEvent{Event: event.Event, Payload: event.Payload}:
			case <-c.closed:
				return
			}
		default:
			select {
			case c.errs <- fmt.Errorf("unsupported ipc message type %q", envelope.Type):
			default:
			}
		}
	}
}
