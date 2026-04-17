package httpx

import "context"

type contextKey string

const correlationIDKey contextKey = "correlation_id"

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

func CorrelationIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(correlationIDKey).(string)
	return value
}
