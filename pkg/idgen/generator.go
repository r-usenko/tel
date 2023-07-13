package idgen

import (
	"context"
	"crypto/rand"

	"go.opentelemetry.io/otel/trace"
)

//crypto (thread safe) generator instead math to avoid collisions in ID

type CryptoIdGenerator struct {
}

func (i *CryptoIdGenerator) NewIDs(_ context.Context) (trace.TraceID, trace.SpanID) {
	tid := trace.TraceID{}
	_, _ = rand.Read(tid[:])
	sid := trace.SpanID{}
	_, _ = rand.Read(sid[:])
	return tid, sid
}

func (i *CryptoIdGenerator) NewSpanID(_ context.Context, _ trace.TraceID) trace.SpanID {
	sid := trace.SpanID{}
	_, _ = rand.Read(sid[:])

	return sid
}
