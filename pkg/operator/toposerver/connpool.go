/*
Copyright 2019 PlanetScale Inc.

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

/*
Package toposerver helps with connecting to Vitess topology for many clusters at the same time.

It maintains at most one topology connection for each global server endpoint,
making it more efficient to talk to topology from many different controllers
concurrently.
*/
package toposerver

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"vitess.io/vitess/go/vt/topo"

	// Register the common topo plugins.
	_ "vitess.io/vitess/go/vt/topo/consultopo"
	_ "vitess.io/vitess/go/vt/topo/etcd2topo"
	_ "vitess.io/vitess/go/vt/topo/zk2topo"
	_ "vitess.io/vitess/go/vt/topo/k8stopo"

	planetscalev2 "planetscale.dev/vitess-operator/pkg/apis/planetscale/v2"
)

const (
	// idleTTL is how long to keep a connection around before closing it.
	// If someone opens the connection before then, the TTL is refreshed.
	idleTTL = 1 * time.Minute

	// gcInterval is how often to check if any connections have exceeded
	// their idleTTL.
	gcInterval = 10 * time.Second

	// connectTimeout is how long to wait synchronously for the connection.
	// We set this to a low value to keep the controller workqueue moving.
	// The controller should requeue to try again after giving other
	// work items a chance to run. The connection attempt will continue in the
	// background and hopefully the connection will be ready the next time the
	// controller reconciles that particular object.
	connectTimeout = 1 * time.Second

	// livenessCheckPeriod is how long to wait between connection liveness checks
	// on cached connections.
	livenessCheckPeriod = 10 * time.Second

	// livenessCheckTimeout is how long to wait for the connection liveness check
	// before considering the connection dead.
	livenessCheckTimeout = 5 * time.Second
)

// pool is the process-wide shared pool of connections.
var pool = &connPool{conns: make(map[planetscalev2.VitessLockserverParams]*Conn)}

var log = logrus.WithField("component", "toposerver.connpool")

func init() {
	// Start the garbage-collection goroutine.
	go func() {
		for {
			time.Sleep(gcInterval)
			pool.gc()
		}
	}()
}

// Open returns a topo server connection for the given params.
// If the returned error is nil, you must call Close() on the returned
// connection when you're done using it.
func Open(ctx context.Context, params planetscalev2.VitessLockserverParams) (*Conn, error) {
	startTime := time.Now()
	defer func() {
		openLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Hold the openMu RLock for as long as we're trying to get a connection,
	// to prevent the connection GC from closing connections.
	// Other Open attempts can happen concurrently, however.
	pool.openMu.RLock()
	defer pool.openMu.RUnlock()

	// Get or start a connection attempt.
	conn := pool.get(params)

	// Wait for the connection attempt to finish.
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	return conn.open(ctx)
}

type connPool struct {
	// conns is the set of active connections to use.
	conns map[planetscalev2.VitessLockserverParams]*Conn

	// deadConns is a list of connections that have gone bad and need to be
	// closed one no one is using them anymore. We can add conns to this list
	// while holding mapMu, but we can only close connections while holding
	// openMu, so we let the GC do the closing.
	deadConns []*Conn

	// mapMu guards all reads and writes of the conns map.
	mapMu sync.Mutex

	// openMu blocks attempts to open connections ("reads")
	// while the garbage collector is closing connections ("writes").
	openMu sync.RWMutex
}

// get returns a connection attempt from the pool,
// creating a new attempt if necessary.
func (p *connPool) get(params planetscalev2.VitessLockserverParams) *Conn {
	pool.mapMu.Lock()
	defer pool.mapMu.Unlock()

	conn := p.conns[params]
	if conn != nil {
		if conn.failed() {
			// If the connect attempt failed, remove it and pretend it wasn't found.
			delete(p.conns, params)
			conn = nil
		} else if conn.succeeded() {
			cacheHits.Inc()
			// Every time we fetch a cached conn, we also check if the connection is
			// still alive. We do this asynchronously because we need a long timeout to
			// avoid false negatives, but we don't want to hold up the caller.
			go p.checkConn(params)
		}
	}
	if conn == nil {
		cacheMisses.Inc()
		// Start a new connection attempt.
		conn = newConn(params)
		p.conns[params] = conn
	}
	return conn
}

// checkConn performs a liveness check on the conection,
// and removes it from the cache if it fails.
func (p *connPool) checkConn(params planetscalev2.VitessLockserverParams) {
	// Take the usual locks as if we are opening the connection like anyone else.
	p.openMu.RLock()
	defer pool.openMu.RUnlock()

	p.mapMu.Lock()
	conn := p.conns[params]
	p.mapMu.Unlock()

	if conn == nil {
		// The conn we were asked to check was removed anyway.
		return
	}
	if !conn.succeeded() {
		// We only check conns that claim to be good.
		return
	}
	if !conn.shouldCheck() {
		// It hasn't been long enough since the last liveness check.
		return
	}

	// Try a simple read operation from global topo.
	ctx, cancel := context.WithTimeout(context.Background(), livenessCheckTimeout)
	defer cancel()

	_, err := conn.Server.GetCellInfoNames(ctx)
	if err == nil || topo.IsErrType(err, topo.NoNode) {
		// The check passed. Nothing to do.
		checkSuccesses.Inc()
		return
	}

	// The connection is bad. Remove it from the cache so a new one will be created.
	log.WithFields(logrus.Fields{
		"implementation": conn.params.Implementation,
		"address":        conn.params.Address,
		"rootPath":       conn.params.RootPath,
	}).Info("cached connection to Vitess topology server failed liveness check")
	checkErrors.Inc()

	p.mapMu.Lock()
	defer p.mapMu.Unlock()

	// Now that we have the map lock, confirm that the entry in the cache is
	// still the same one we checked.
	if p.conns[params] != conn {
		// Someone else already removed or replaced it.
		return
	}

	// Send it to the deadConns list so the GC will close it while holding the
	// openMu write lock.
	delete(p.conns, params)
	p.deadConns = append(p.deadConns, conn)
}

// gc closes any connections that have outlived their TTL.
func (p *connPool) gc() {
	p.openMu.Lock()
	defer p.openMu.Unlock()

	p.mapMu.Lock()
	defer p.mapMu.Unlock()

	var activeRefs int64
	for params, conn := range p.conns {
		// We hold the openMu write lock, so no one is trying to open a connection.
		// The only thing we might race with is callers decrementing the refCount,
		// which is fine. What matters is that no one will race to increment it,
		// which could reverse a decision we had already made to close the connection.
		conn.mu.Lock()
		activeRefs += conn.refCount
		if conn.failed() {
			// The connection attempt failed, so remove it without trying to close it.
			delete(p.conns, params)
		} else if conn.refCount <= 0 && time.Since(conn.lastOpened) > idleTTL {
			log.WithFields(logrus.Fields{
				"implementation": params.Implementation,
				"address":        params.Address,
				"rootPath":       params.RootPath,
			}).Info("closing connection to Vitess topology server due to idle TTL")
			disconnects.WithLabelValues(reasonIdle).Inc()

			conn.Server.Close()
			delete(p.conns, params)
		}
		conn.mu.Unlock()
	}
	connCount.WithLabelValues(connStateActive).Set(float64(len(p.conns)))
	connRefCount.WithLabelValues(connStateActive).Set(float64(activeRefs))

	// Clean up bad conns once they're no longer being used.
	// Make a list of bad conns that still have refs (we need to keep waiting).
	var deadRefs int64
	stillUsed := make([]*Conn, 0, len(p.deadConns))
	for _, conn := range p.deadConns {
		conn.mu.Lock()
		deadRefs += conn.refCount
		if conn.refCount <= 0 {
			log.WithFields(logrus.Fields{
				"implementation": conn.params.Implementation,
				"address":        conn.params.Address,
				"rootPath":       conn.params.RootPath,
			}).Info("closing connection to Vitess topology server due to liveness check failure")
			disconnects.WithLabelValues(reasonDead).Inc()

			conn.Server.Close()
		} else {
			log.WithFields(logrus.Fields{
				"implementation": conn.params.Implementation,
				"address":        conn.params.Address,
				"rootPath":       conn.params.RootPath,
			}).Warning("cached connection to Vitess topology server failed liveness check but is still in use")

			stillUsed = append(stillUsed, conn)
		}
		conn.mu.Unlock()
	}
	p.deadConns = stillUsed
	connCount.WithLabelValues(connStateDead).Set(float64(len(p.deadConns)))
	connRefCount.WithLabelValues(connStateDead).Set(float64(deadRefs))
}

// Conn represents a connection to a topology server.
type Conn struct {
	// Server is embedded anonymously, so all the topo.Server methods are available.
	// Do not try to read this until after connectDone is closed.
	// After that, it will be nil if connectErr is not nil.
	*topo.Server

	// connectDone is closed when the background connection attempt finishes.
	connectDone chan struct{}
	// connectErr is the result of the background connection attempt.
	// Do not try to read this until after connectDone is closed.
	connectErr error

	mu          sync.Mutex
	refCount    int64
	lastOpened  time.Time
	lastChecked time.Time
	params      planetscalev2.VitessLockserverParams
}

// newConn starts a new connection attempt in the background.
// It returns a Conn, which can be used to wait for the attempt.
func newConn(params planetscalev2.VitessLockserverParams) *Conn {
	now := time.Now()
	c := &Conn{
		params:      params,
		connectDone: make(chan struct{}),
		lastOpened:  now,
		lastChecked: now,
	}

	go func() {
		connLog := log.WithFields(logrus.Fields{
			"implementation": params.Implementation,
			"address":        params.Address,
			"rootPath":       params.RootPath,
		})
		connLog.Info("connecting to Vitess topology server")

		startTime := time.Now()
		defer func() {
			connectLatency.Observe(time.Since(startTime).Seconds())
		}()

		// OpenServer has a built-in timeout that's not configurable.
		// TODO(enisoc): Upstream a change to make the timeout configurable.
		c.Server, c.connectErr = topo.OpenServer(params.Implementation, params.Address, params.RootPath)
		if c.connectErr == nil {
			connLog.Info("successfully connected to Vitess topology server")
			connectSuccesses.Inc()
		} else {
			connLog.WithField("err", c.connectErr).Warning("failed to connect to Vitess topology server")
			connectErrors.Inc()
		}
		close(c.connectDone)
	}()

	return c
}

// open waits for the connection attempt to succeed or fail.
func (c *Conn) open(ctx context.Context) (conn *Conn, err error) {
	c.mu.Lock()
	c.refCount++
	c.lastOpened = time.Now()
	c.mu.Unlock()

	defer func() {
		// If we return an error, the caller is not expected to call Close().
		// We need to decrement the refCount ourselves.
		if err != nil {
			c.mu.Lock()
			c.refCount--
			c.mu.Unlock()
		}
	}()

	select {
	case <-c.connectDone:
		if c.connectErr != nil {
			return nil, c.connectErr
		}
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// shouldCheck returns true if it's been long enough since the last liveness
// check that it's time to do another one.
func (c *Conn) shouldCheck() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.lastChecked) >= livenessCheckPeriod {
		c.lastChecked = time.Now()
		return true
	}
	return false
}

// failed returns true if and only if the connection attempt has finished with an error.
// It does not wait.
func (c *Conn) failed() bool {
	select {
	case <-c.connectDone:
		return c.connectErr != nil
	default:
		return false
	}
}

// succeeded returns true if and only if the connection attempt has succeeded.
// It does not wait.
func (c *Conn) succeeded() bool {
	select {
	case <-c.connectDone:
		return c.connectErr == nil
	default:
		return false
	}
}

// Close should be called when you're done using the toposerver connection for now.
// This shadows topo.Server.Close() so that the underlying connection doesn't
// actually get closed when the user calls Conn.Close().
func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.refCount--
}
