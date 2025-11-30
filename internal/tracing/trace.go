package tracing

import (
	"time"

	"github.com/google/uuid"
)

const TraceKey string = "trace"

type Trace struct {
	ID        string
	StartTime time.Time
}

func NewTrace() Trace {
	return Trace{
		ID:        uuid.NewString(),
		StartTime: time.Now(),
	}
}
