package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-pantheon/fabrica-net/xnet"
	"github.com/go-pantheon/fabrica-util/xsync"
)

type CreateTunnelFunc func(ctx context.Context, tunnelType int32, objectId int64) (xnet.Tunnel, error)

type tunnelManager struct {
	mu sync.RWMutex

	tunnelGroups map[int32]map[int64]xnet.Tunnel // TunnelType -> oid -> tunnel
}

const defaultTunnelGroupSize = 32

func newTunnelManager(tunnelGroupSize int) *tunnelManager {
	if tunnelGroupSize <= 0 || tunnelGroupSize > 512 {
		tunnelGroupSize = defaultTunnelGroupSize
	}

	return &tunnelManager{
		tunnelGroups: make(map[int32]map[int64]xnet.Tunnel, tunnelGroupSize),
	}
}

func (h *tunnelManager) tunnel(tp int32, oid int64) xnet.Tunnel {
	h.mu.RLock()
	defer h.mu.RUnlock()

	tg, ok := h.tunnelGroups[tp]
	if !ok {
		return nil
	}

	t, ok := tg[oid]
	if !ok || t.OnStopping() {
		return nil
	}

	return t
}

func (h *tunnelManager) createTunnel(ctx context.Context, tp int32, oid int64, initCap int, create CreateTunnelFunc) (xnet.Tunnel, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	tg, ok := h.tunnelGroups[tp]
	if !ok {
		tg = make(map[int64]xnet.Tunnel, initCap)
		h.tunnelGroups[tp] = tg
	}

	t, ok := tg[oid]
	if ok && !t.OnStopping() {
		return t, nil
	}

	t, err := create(ctx, tp, oid)
	if err != nil {
		return nil, err
	}

	tg[oid] = t

	return t, nil
}

func (h *tunnelManager) stop(ctx context.Context) (err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for tp, tg := range h.tunnelGroups {
		delete(h.tunnelGroups, tp)

		wg := sync.WaitGroup{}

		for _, t := range tg {
			wg.Add(1)

			xsync.Go(fmt.Sprintf("tunnelManager.stopTunnel.%d", t.Type()), func() error {
				defer wg.Done()
				return t.Stop(ctx)
			})
		}

		wg.Wait()
	}

	return nil
}
