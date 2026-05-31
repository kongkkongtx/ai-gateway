package balancer

import (
	"context"
	"log/slog"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Upstream struct {
	Name     string
	Endpoint string
	Provider string
	Weight   int
	Healthy  atomic.Bool
	conns    atomic.Int64
}

func (u *Upstream) Conns() int64 { return u.conns.Load() }

type Balancer struct {
	mu          sync.RWMutex
	upstreams   []*Upstream
	totalWeight int
	logger      *slog.Logger
}

func New(logger *slog.Logger) *Balancer {
	return &Balancer{logger: logger}
}

func (b *Balancer) AddUpstream(name, endpoint, provider string, weight int) *Upstream {
	b.mu.Lock()
	defer b.mu.Unlock()
	u := &Upstream{Name: name, Endpoint: endpoint, Provider: provider, Weight: weight}
	u.Healthy.Store(true)
	b.upstreams = append(b.upstreams, u)
	b.totalWeight += weight
	b.logger.Info("added upstream", "name", name, "endpoint", endpoint, "weight", weight)
	return u
}

func (b *Balancer) RemoveUpstream(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, u := range b.upstreams {
		if u.Name == name {
			b.totalWeight -= u.Weight
			b.upstreams = append(b.upstreams[:i], b.upstreams[i+1:]...)
			b.logger.Info("removed upstream", "name", name)
			return true
		}
	}
	return false
}

func (b *Balancer) Next() *Upstream {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.upstreams) == 0 {
		return nil
	}
	type candidate struct {
		u      *Upstream
		weight int
	}
	var candidates []candidate
	totalHealthyWeight := 0
	for _, u := range b.upstreams {
		if u.Healthy.Load() {
			candidates = append(candidates, candidate{u: u, weight: u.Weight})
			totalHealthyWeight += u.Weight
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	n := rand.Intn(totalHealthyWeight)
	for _, c := range candidates {
		n -= c.weight
		if n < 0 {
			return c.u
		}
	}
	return candidates[0].u
}

func (b *Balancer) NextLeastConn() *Upstream {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var best *Upstream
	bestConns := int64(1<<63 - 1)
	for _, u := range b.upstreams {
		if u.Healthy.Load() {
			conns := u.conns.Load()
			if conns < bestConns {
				bestConns = conns
				best = u
			}
		}
	}
	return best
}

func (b *Balancer) StartHealthChecks(ctx context.Context, interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.checkAll(timeout)
			}
		}
	}()
	b.logger.Info("health checks started", "interval", interval)
}

func (b *Balancer) checkAll(timeout time.Duration) {
	b.mu.RLock()
	upstreams := make([]*Upstream, len(b.upstreams))
	copy(upstreams, b.upstreams)
	b.mu.RUnlock()
	for _, u := range upstreams {
		healthy := b.check(u.Endpoint, timeout)
		wasHealthy := u.Healthy.Load()
		u.Healthy.Store(healthy)
		if wasHealthy != healthy {
			if healthy {
				b.logger.Info("upstream recovered", "name", u.Name, "endpoint", u.Endpoint)
			} else {
				b.logger.Warn("upstream unhealthy", "name", u.Name, "endpoint", u.Endpoint)
			}
		}
	}
}

func (b *Balancer) check(endpoint string, timeout time.Duration) bool {
	host := endpoint
	useTLS := false
	if strings.HasPrefix(host, "https://") {
		host = host[8:]
		useTLS = true
	} else if strings.HasPrefix(host, "http://") {
		host = host[7:]
	}
	for i := 0; i < len(host); i++ {
		if host[i] == '/' {
			host = host[:i]
			break
		}
	}
	if !strings.Contains(host, ":") {
		if useTLS {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}
	conn, err := net.DialTimeout("tcp", host, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (b *Balancer) Upstreams() []*Upstream {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]*Upstream, len(b.upstreams))
	copy(result, b.upstreams)
	return result
}
