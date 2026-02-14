package logging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// Redactor interface for types that support redaction
type Redactor interface {
	Redact() string
}

// RedactedServer returns a server logging middleware that respects redaction.
func RedactedServer(logger log.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			var (
				component string
				operation string
			)

			if info, ok := transport.FromServerContext(ctx); ok {
				component = info.Kind().String()
				operation = info.Operation()
			}

			startTime := time.Now()
			reply, err = handler(ctx, req)
			latency := time.Since(startTime).Seconds()

			// Skip logging for successful heartbeats (too noisy)
			if err == nil && strings.HasSuffix(operation, "/Heartbeat") {
				return
			}

			level, stack := extractError(err)
			args := extractArgs(req)

			if logErr := log.WithContext(ctx, logger).Log(level,
				"kind", "server",
				"component", component,
				"operation", operation,
				"args", args,
				"code", extractCode(err),
				"reason", extractReason(err),
				"stack", stack,
				"latency", latency,
			); logErr != nil {
				fmt.Fprintf(os.Stderr, "redacted logging failed: %v\n", logErr)
			}

			return
		}
	}
}

func extractArgs(req interface{}) string {
	if req == nil {
		return ""
	}
	if r, ok := req.(Redactor); ok {
		return r.Redact()
	}
	str := fmt.Sprintf("%+v", req)
	if len(str) > 512 {
		return str[:512] + "...[truncated]"
	}
	return str
}

func extractError(err error) (log.Level, string) {
	if err != nil {
		return log.LevelError, fmt.Sprintf("%+v", err)
	}
	return log.LevelInfo, ""
}

func extractCode(err error) int32 {
	if se := errors.FromError(err); se != nil {
		return int32(se.Code)
	}
	return 0
}

func extractReason(err error) string {
	if se := errors.FromError(err); se != nil {
		return se.Reason
	}
	return ""
}
