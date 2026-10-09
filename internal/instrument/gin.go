package instrument

import (
	"strconv"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

const (
	spanKey         = "instrument.span"
	RequestIDHeader = "X-Request-ID"
)

var requestCounter atomic.Uint64

// Middleware gives every request a correlation ID (req-N, echoed in
// X-Request-ID) and records its start and final HTTP status.
func Middleware(sink Sink) gin.HandlerFunc {
	return func(c *gin.Context) {
		num := requestCounter.Add(1)
		id := "req-" + strconv.FormatUint(num, 10)
		c.Header(RequestIDHeader, id)

		span := NewSpan(sink, id, num)
		c.Set(spanKey, span)
		span.Started(c.Request.Method, c.FullPath())
		c.Next()
		span.Finished(c.Writer.Status())
	}
}

// SpanFrom returns the request's span, or nil when instrumentation is off.
func SpanFrom(c *gin.Context) *Span {
	if v, ok := c.Get(spanKey); ok {
		if s, ok := v.(*Span); ok {
			return s
		}
	}
	return nil
}
