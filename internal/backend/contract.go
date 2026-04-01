package backend

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	APIVersion = "1"

	HeartbeatFrequency = 10 * time.Second
	HeartbeatGrace     = 3
	SessionTimeout     = 30 * time.Second
)

type Operation string

const (
	BeginSessionOperation Operation = "beginsession"
	StatusOperation       Operation = "status"
	PingOperation         Operation = "ping"
	EndSessionOperation   Operation = "endsession"
)

func (o Operation) Path() string {
	return fmt.Sprintf("/api/console/%s/%s", APIVersion, o)
}

type SupportSessionState string

const (
	SupportSessionOpen   SupportSessionState = "open"
	SupportSessionActive SupportSessionState = "active"
	SupportSessionClosed SupportSessionState = "closed"
)

func (s SupportSessionState) Validate() error {
	switch s {
	case SupportSessionOpen, SupportSessionActive, SupportSessionClosed:
		return nil
	default:
		return fmt.Errorf("unsupported session state %q", s)
	}
}

type SupportSession struct {
	Status    SupportSessionState `json:"status"`
	PIN       string              `json:"pin"`
	IPAddress string              `json:"ipAddress"`
}

func (s SupportSession) Validate() error {
	var errs []error

	if err := s.Status.Validate(); err != nil {
		errs = append(errs, err)
	}

	if strings.TrimSpace(s.PIN) == "" {
		errs = append(errs, errors.New("session pin must not be empty"))
	}

	if strings.TrimSpace(s.IPAddress) == "" {
		errs = append(errs, errors.New("session ipAddress must not be empty"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// StartSupportSessionRequest intentionally has no fields yet. The current
// contract trusts the active VPN connection and the OpenAPI draft keeps the
// request body empty until the long-term identity model is expanded.
type StartSupportSessionRequest struct{}

type SessionStatusRequest struct {
	PIN string `json:"pin"`
}

func (r SessionStatusRequest) Validate() error {
	return validatePIN(r.PIN)
}

type SessionHeartbeatRequest struct {
	PIN string `json:"pin"`
}

func (r SessionHeartbeatRequest) Validate() error {
	return validatePIN(r.PIN)
}

type EndSupportSessionRequest struct {
	PIN string `json:"pin"`
}

func (r EndSupportSessionRequest) Validate() error {
	return validatePIN(r.PIN)
}

type StartSupportSessionResponse struct {
	Session SupportSession `json:"session"`
}

func (r StartSupportSessionResponse) Validate() error {
	return r.Session.Validate()
}

type SessionStatusResponse struct {
	Session SupportSession `json:"session"`
}

func (r SessionStatusResponse) Validate() error {
	return r.Session.Validate()
}

// GenericAckResponse is currently an empty object by contract.
type GenericAckResponse struct{}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r ErrorResponse) Validate() error {
	var errs []error

	if strings.TrimSpace(r.Code) == "" {
		errs = append(errs, errors.New("error code must not be empty"))
	}

	if strings.TrimSpace(r.Message) == "" {
		errs = append(errs, errors.New("error message must not be empty"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func validatePIN(pin string) error {
	if strings.TrimSpace(pin) == "" {
		return errors.New("pin must not be empty")
	}

	return nil
}
