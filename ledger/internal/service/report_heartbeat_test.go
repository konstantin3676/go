package service

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"
)

type heartbeatLog struct {
	bytes.Buffer
	ticks chan struct{}
}

func (w *heartbeatLog) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data)
	if strings.Contains(string(data), "in progress") {
		select {
		case w.ticks <- struct{}{}:
		default:
		}
	}
	return n, err
}

func TestReportHeartbeatLifecycle(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "canceled"}[canceled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			output := &heartbeatLog{ticks: make(chan struct{}, 1)}
			stop := startReportHeartbeat(ctx, time.Millisecond, log.New(output, "", 0))
			select {
			case <-output.ticks:
			case <-time.After(2 * time.Second):
				stop()
				t.Fatal("heartbeat did not tick")
			}
			if canceled {
				cancel()
			}
			stop()
			want := "calculation finished"
			if canceled {
				want = "calculation interrupted (ctx.Done): context canceled"
			}
			if !strings.Contains(output.String(), "report calculation in progress") ||
				!strings.Contains(output.String(), "report heartbeat stopped; "+want) {
				t.Fatalf("logs = %s", output.String())
			}
		})
	}
}
