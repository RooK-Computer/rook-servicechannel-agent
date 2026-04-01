package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	if _, err := NewClient("://missing", nil); err == nil {
		t.Fatal("NewClient returned nil error for invalid base URL")
	}
}

func TestBeginSessionUsesConfiguredEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}

		if r.URL.Path != "/api/console/1/beginsession" {
			t.Fatalf("path = %s, want /api/console/1/beginsession", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(StartSupportSessionResponse{
			Session: SupportSession{
				Status:    SupportSessionOpen,
				PIN:       "1234",
				IPAddress: "10.8.0.2",
			},
		}); err != nil {
			t.Fatalf("Encode() returned error: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}

	response, err := client.BeginSession(context.Background(), StartSupportSessionRequest{})
	if err != nil {
		t.Fatalf("BeginSession() returned error: %v", err)
	}

	if response.Session.PIN != "1234" {
		t.Fatalf("PIN = %q, want 1234", response.Session.PIN)
	}
}

func TestSessionScopedOperationsSendPIN(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		call     func(Client) error
		wantPIN  string
		status   int
		response string
	}{
		{
			name:    "status",
			path:    "/api/console/1/status",
			wantPIN: "1234",
			call: func(client Client) error {
				_, err := client.GetSessionStatus(context.Background(), SessionStatusRequest{PIN: "1234"})
				return err
			},
			status:   http.StatusOK,
			response: `{"session":{"status":"active","pin":"1234","ipAddress":"10.8.0.3"}}`,
		},
		{
			name:    "ping",
			path:    "/api/console/1/ping",
			wantPIN: "1234",
			call: func(client Client) error {
				_, err := client.SendSessionHeartbeat(context.Background(), SessionHeartbeatRequest{PIN: "1234"})
				return err
			},
			status:   http.StatusOK,
			response: `{}`,
		},
		{
			name:    "endsession",
			path:    "/api/console/1/endsession",
			wantPIN: "1234",
			call: func(client Client) error {
				_, err := client.EndSession(context.Background(), EndSupportSessionRequest{PIN: "1234"})
				return err
			},
			status:   http.StatusOK,
			response: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.path {
					t.Fatalf("path = %s, want %s", r.URL.Path, tt.path)
				}

				var payload map[string]string
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("Decode() returned error: %v", err)
				}

				if payload["pin"] != tt.wantPIN {
					t.Fatalf("pin = %q, want %q", payload["pin"], tt.wantPIN)
				}

				w.WriteHeader(tt.status)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatalf("NewClient() returned error: %v", err)
			}

			if err := tt.call(client); err != nil {
				t.Fatalf("call() returned error: %v", err)
			}
		})
	}
}

func TestClientReturnsStructuredRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"session_timeout","message":"session already closed"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewClient() returned error: %v", err)
	}

	_, err = client.GetSessionStatus(context.Background(), SessionStatusRequest{PIN: "1234"})
	if err == nil {
		t.Fatal("GetSessionStatus() returned nil error")
	}

	var requestErr *RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error type = %T, want *RequestError", err)
	}

	if requestErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("StatusCode = %d, want %d", requestErr.StatusCode, http.StatusInternalServerError)
	}

	if requestErr.Code != "session_timeout" {
		t.Fatalf("Code = %q, want session_timeout", requestErr.Code)
	}
}
