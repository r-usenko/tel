package idgen_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tel-io/tel/v2/pkg/idgen"
	strace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func testCollision(t *testing.T, tr trace.Tracer) {
	const count = 10000000

	wg := new(sync.WaitGroup)
	wg.Add(count)
	var muLock = new(sync.Mutex)
	var tList = make(map[string]int, count)
	var sList = make(map[string]int, count)

	ctx := context.Background()

	for i := int64(0); i < count; i++ {
		go func(i int64) {
			defer wg.Done()

			_, s := tr.Start(ctx, strconv.FormatInt(i, 10))
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

	assert.Equal(t, collisionCount, 0)
}

func TestCollisionOtelGenerator(t *testing.T) {
	tr := strace.NewTracerProvider()

	testCollision(t, tr.Tracer("otel"))
}

func TestCollisionIdGenerator(t *testing.T) {
	tr := strace.NewTracerProvider(
		strace.WithIDGenerator(new(idgen.CryptoIdGenerator)),
	)

	testCollision(t, tr.Tracer("idgen"))
}
