/*
Copyright 2026 PlanetScale Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package toposerver

import (
	"context"
	"testing"
	"time"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

// TestOpenFailsForUnreachableServer verifies that a connection attempt to a
// lockserver that cannot be reached is reported as failed, rather than being
// cached as a usable connection whose every topo call then hangs until its
// deadline. Newer topo clients (etcd client v3.7+) connect lazily, so
// topo.OpenServer alone no longer detects an unreachable server.
func TestOpenFailsForUnreachableServer(t *testing.T) {
	params := planetscalev2.VitessLockserverParams{
		Implementation: "etcd2",
		// Nothing listens on port 1, so the connection is refused without any
		// DNS lookup.
		Address:  "127.0.0.1:1",
		RootPath: "/vitess/toposerver-test",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if conn, err := Open(ctx, params); err == nil {
		conn.Close()
		t.Fatalf("Open() returned a connection to an unreachable server; want an error")
	}

	// Open only waits connectTimeout for the background attempt, so wait for
	// the attempt itself to finish and confirm that it failed.
	conn := pool.get(params)
	select {
	case <-conn.connectDone:
	case <-ctx.Done():
		t.Fatalf("connection attempt to %v did not finish: %v", params.Address, ctx.Err())
	}
	if conn.connectErr == nil {
		t.Fatalf("connection attempt to unreachable server %v succeeded; want an error", params.Address)
	}
	if conn.Server != nil {
		t.Fatalf("failed connection attempt still holds a topo.Server")
	}
	t.Logf("connection attempt failed as expected: %v", conn.connectErr)
}
