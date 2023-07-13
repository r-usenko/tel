package idgen_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tel-io/tel/v2/pkg/idgen"
	strace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func testCollision(tr trace.Tracer, count int) int {
	wg := new(sync.WaitGroup)
	wg.Add(count)
	var muLock = new(sync.Mutex)
	var tList = make(map[string]int, count)
	var sList = make(map[string]int, count)

	ctx := context.Background()

	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			_, s := tr.Start(ctx, fmt.Sprintf("test-%d", i))
			sCtx := s.SpanContext()
			tid := sCtx.TraceID().String()
			sid := sCtx.SpanID().String()

			muLock.Lock()
			tList[tid]++
			sList[sid]++
			muLock.Unlock()
		}(i)

	}

	wg.Wait()

	var collisionCount int
	for _, v := range tList {
		if v > 1 {
			collisionCount += v
		}
	}
	for _, v := range sList {
		if v > 1 {
			collisionCount += v
		}
	}

	return collisionCount
}

func BenchmarkOtel(b *testing.B) {
	tr := strace.NewTracerProvider().Tracer("otel")

	for i := 0; i < b.N; i++ {
		testCollision(tr, 10000)
	}
}

func BenchmarkIdGen(b *testing.B) {
	tr := strace.NewTracerProvider(
		strace.WithIDGenerator(new(idgen.CryptoIdGenerator)),
	).Tracer("idgen")

	for i := 0; i < b.N; i++ {
		testCollision(tr, 10000)
	}
}

func TestCollisionOtelGenerator(t *testing.T) {
	tr := strace.NewTracerProvider().Tracer("otel")

	assert.Equal(t, testCollision(tr, 1000000), 0)
}

func TestCollisionIdGenerator(t *testing.T) {
	tr := strace.NewTracerProvider(
		strace.WithIDGenerator(new(idgen.CryptoIdGenerator)),
	).Tracer("idgen")

	assert.Equal(t, testCollision(tr, 1000000), 0)
}
