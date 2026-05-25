package fcm

import "context"

type ResultStatus string

const (
	ResultStatusDelivered ResultStatus = "delivered"
	ResultStatusFailed    ResultStatus = "failed"
	ResultStatusInvalid   ResultStatus = "invalid"
)

type ResultEnvelope struct {
	CorrelationID string
	Service       string
	Status        ResultStatus
	GatewayID     string
	HTTPStatus    int
	ErrorCode     string
	ErrorMessage  string
	LatencyMS     int64
	Timestamp     int64
}

type ResultEmitter interface {
	Emit(ctx context.Context, r ResultEnvelope) error
}
