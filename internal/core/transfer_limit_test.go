package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTransferLimitWaitAndLiveChange(t *testing.T) {
	l := &TransferLimiter{}
	l.Set(10000)
	ctx := WithTransferLimiter(context.Background(), l)
	start := time.Now()
	if err := waitTransfer(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 90*time.Millisecond {
		t.Fatal("limit not enforced")
	}
	l.Set(1)
	done := make(chan error, 1)
	go func() { done <- waitTransfer(ctx, 10000) }()
	l.Set(0)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("live change did not wake wait")
	}
	l.Set(1)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := waitTransfer(cancelCtx, 10000); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := waitTransfer(WithTransferLimiter(context.Background(), &TransferLimiter{}), 10000); err != nil {
		t.Fatal("other tab affected", err)
	}
}
