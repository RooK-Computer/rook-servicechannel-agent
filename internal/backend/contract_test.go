package backend

import "testing"

func TestOperationPath(t *testing.T) {
	tests := map[string]struct {
		operation Operation
		want      string
	}{
		"begin session": {operation: BeginSessionOperation, want: "/api/console/1/beginsession"},
		"status":        {operation: StatusOperation, want: "/api/console/1/status"},
		"ping":          {operation: PingOperation, want: "/api/console/1/ping"},
		"end session":   {operation: EndSessionOperation, want: "/api/console/1/endsession"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.operation.Path(); got != tt.want {
				t.Fatalf("Path() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSupportSessionStateValidate(t *testing.T) {
	validStates := []SupportSessionState{
		SupportSessionOpen,
		SupportSessionActive,
		SupportSessionClosed,
	}

	for _, state := range validStates {
		if err := state.Validate(); err != nil {
			t.Fatalf("Validate() for %q returned error: %v", state, err)
		}
	}

	if err := SupportSessionState("unknown").Validate(); err == nil {
		t.Fatal("Validate() for unknown state returned nil error")
	}
}

func TestSupportSessionValidate(t *testing.T) {
	session := SupportSession{
		Status:    SupportSessionOpen,
		PIN:       "1234",
		IPAddress: "10.0.0.5",
	}

	if err := session.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if err := (SupportSession{}).Validate(); err == nil {
		t.Fatal("Validate() for zero-value session returned nil error")
	}
}

func TestSessionScopedRequestsRequirePIN(t *testing.T) {
	tests := map[string]interface {
		Validate() error
	}{
		"status":    SessionStatusRequest{},
		"ping":      SessionHeartbeatRequest{},
		"end":       EndSupportSessionRequest{},
		"status ok": SessionStatusRequest{PIN: "1234"},
	}

	for name, request := range tests {
		t.Run(name, func(t *testing.T) {
			err := request.Validate()
			if name == "status ok" {
				if err != nil {
					t.Fatalf("Validate() returned error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Validate() returned nil error")
			}
		})
	}
}

func TestResponsesValidateEmbeddedData(t *testing.T) {
	session := SupportSession{
		Status:    SupportSessionActive,
		PIN:       "9876",
		IPAddress: "10.8.0.7",
	}

	if err := (StartSupportSessionResponse{Session: session}).Validate(); err != nil {
		t.Fatalf("StartSupportSessionResponse.Validate() returned error: %v", err)
	}

	if err := (SessionStatusResponse{Session: session}).Validate(); err != nil {
		t.Fatalf("SessionStatusResponse.Validate() returned error: %v", err)
	}

	if err := (ErrorResponse{Code: "backend_failure", Message: "failure"}).Validate(); err != nil {
		t.Fatalf("ErrorResponse.Validate() returned error: %v", err)
	}
}
