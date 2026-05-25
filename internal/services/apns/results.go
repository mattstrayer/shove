package apns

import "context"

// ResultStatus mirrors queueredis.ResultStatus but is redeclared locally
// to keep the services/apns package free of a direct queue/redis import.
// The bridge type is converted at the seam in cmd/.
type ResultStatus string

const (
	ResultStatusDelivered ResultStatus = "delivered"
	ResultStatusFailed    ResultStatus = "failed"
	ResultStatusInvalid   ResultStatus = "invalid"
)

// ResultEnvelope is the in-package shape; cmd wires it to queue/redis.Result.
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

// ResultEmitter is the abstraction APNS uses to publish results.
// cmd injects an adapter that forwards to queue/redis.ResultEmitter.
type ResultEmitter interface {
	Emit(ctx context.Context, r ResultEnvelope) error
}
