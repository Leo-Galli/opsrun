// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"testing"
)

// BenchmarkSSHTransportSequentialSessions measures the cost of opening and running many
// sequential sessions over a single pooled connection.
func BenchmarkSSHTransportSequentialSessions(b *testing.B) {
	ts := newTestServer(b)
	tr := testTransport(b, ts, func(cfg *PoolConfig) { cfg.PerHostMaxSessions = 200 })
	host := testHost(b, ts)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess, err := tr.Open(ctx, host)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := sess.Run(ctx, "echo:bench", RunOptions{}); err != nil {
			b.Fatal(err)
		}
		sess.Close() //nolint:errcheck
	}
}

// BenchmarkSSHTransportConcurrentSessions simulates hundreds of concurrent sessions
// multiplexed over a single pooled connection.
func BenchmarkSSHTransportConcurrentSessions(b *testing.B) {
	ts := newTestServer(b)
	tr := testTransport(b, ts, func(cfg *PoolConfig) { cfg.PerHostMaxSessions = 300 })
	host := testHost(b, ts)
	ctx := context.Background()

	b.SetParallelism(200)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sess, err := tr.Open(ctx, host)
			if err != nil {
				b.Fatal(err)
			}
			if _, err := sess.Run(ctx, "echo:bench", RunOptions{}); err != nil {
				b.Fatal(err)
			}
			sess.Close() //nolint:errcheck
		}
	})
}
