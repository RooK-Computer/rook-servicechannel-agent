package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	rooklogging "rook-servicechannel-agent/internal/logging"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	baseURL    *url.URL
	httpClient HTTPClient
	logger     *slog.Logger
}

func NewClient(baseURL string, httpClient HTTPClient) (Client, error) {
	return NewClientWithLogger(baseURL, httpClient, nil)
}

func NewClientWithLogger(baseURL string, httpClient HTTPClient, logger *slog.Logger) (Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return Client{}, fmt.Errorf("parse backend base URL: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Client{}, errors.New("backend base URL must include scheme and host")
	}

	return Client{
		baseURL:    parsedURL,
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

func (c Client) BeginSession(ctx context.Context, request StartSupportSessionRequest) (StartSupportSessionResponse, error) {
	response, err := performJSON[StartSupportSessionRequest, StartSupportSessionResponse](ctx, c, BeginSessionOperation, request)
	if err != nil {
		return StartSupportSessionResponse{}, err
	}
	return response, nil
}

func (c Client) GetSessionStatus(ctx context.Context, request SessionStatusRequest) (SessionStatusResponse, error) {
	if err := request.Validate(); err != nil {
		return SessionStatusResponse{}, err
	}

	response, err := performJSON[SessionStatusRequest, SessionStatusResponse](ctx, c, StatusOperation, request)
	if err != nil {
		return SessionStatusResponse{}, err
	}
	return response, nil
}

func (c Client) SendSessionHeartbeat(ctx context.Context, request SessionHeartbeatRequest) (GenericAckResponse, error) {
	if err := request.Validate(); err != nil {
		return GenericAckResponse{}, err
	}

	response, err := performJSON[SessionHeartbeatRequest, GenericAckResponse](ctx, c, PingOperation, request)
	if err != nil {
		return GenericAckResponse{}, err
	}
	return response, nil
}

func (c Client) EndSession(ctx context.Context, request EndSupportSessionRequest) (GenericAckResponse, error) {
	if err := request.Validate(); err != nil {
		return GenericAckResponse{}, err
	}

	response, err := performJSON[EndSupportSessionRequest, GenericAckResponse](ctx, c, EndSessionOperation, request)
	if err != nil {
		return GenericAckResponse{}, err
	}
	return response, nil
}

func (c Client) endpoint(operation Operation) string {
	return c.baseURL.ResolveReference(&url.URL{Path: operation.Path()}).String()
}

type RequestError struct {
	Operation  Operation
	StatusCode int
	Code       string
	Message    string
	Cause      error
}

func (e *RequestError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Code != "" || e.Message != "" {
		return fmt.Sprintf("%s failed with status %d: %s (%s)", e.Operation, e.StatusCode, e.Message, e.Code)
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s failed: %v", e.Operation, e.Cause)
	}

	return fmt.Sprintf("%s failed with status %d", e.Operation, e.StatusCode)
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func performJSON[Req any, Resp any](ctx context.Context, client Client, operation Operation, request Req) (Resp, error) {
	var zero Resp

	body, err := json.Marshal(request)
	if err != nil {
		return zero, fmt.Errorf("marshal %s request: %w", operation, err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint(operation), bytes.NewReader(body))
	if err != nil {
		return zero, fmt.Errorf("create %s request: %w", operation, err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	if rooklogging.DebugEnabled(client.logger) {
		client.logger.Debug("backend request",
			"operation", operation,
			"method", http.MethodPost,
			"url", httpRequest.URL.String(),
			"body", rooklogging.JSONBytes(body),
		)
	}

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		if rooklogging.DebugEnabled(client.logger) {
			client.logger.Debug("backend request failed",
				"operation", operation,
				"url", httpRequest.URL.String(),
				"error", err,
			)
		}
		return zero, &RequestError{
			Operation: operation,
			Cause:     err,
		}
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return zero, fmt.Errorf("read %s response: %w", operation, err)
	}

	if rooklogging.DebugEnabled(client.logger) {
		client.logger.Debug("backend response",
			"operation", operation,
			"status_code", httpResponse.StatusCode,
			"url", httpRequest.URL.String(),
			"body", rooklogging.JSONBytes(responseBody),
		)
	}

	if httpResponse.StatusCode != http.StatusOK {
		requestErr := &RequestError{
			Operation:  operation,
			StatusCode: httpResponse.StatusCode,
		}

		if len(responseBody) > 0 {
			var errorResponse ErrorResponse
			if err := json.Unmarshal(responseBody, &errorResponse); err == nil {
				requestErr.Code = errorResponse.Code
				requestErr.Message = errorResponse.Message
			} else {
				requestErr.Cause = fmt.Errorf("decode error response: %w", err)
			}
		}

		return zero, requestErr
	}

	if len(responseBody) == 0 {
		return zero, nil
	}

	var response Resp
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return zero, fmt.Errorf("decode %s response: %w", operation, err)
	}

	if validator, ok := any(response).(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return zero, fmt.Errorf("validate %s response: %w", operation, err)
		}
	}

	return response, nil
}
