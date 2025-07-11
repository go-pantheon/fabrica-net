package server

import (
	"context"

	"github.com/go-pantheon/fabrica-net/xnet"
)

type Inspector func(InspectorFunc) InspectorFunc

type InspectorFunc func(ctx context.Context, w xnet.Worker) error

func emptyInspector(f InspectorFunc) InspectorFunc {
	return f
}

func EmptyInspectorFunc(_ context.Context, _ xnet.Worker) error {
	return nil
}

func Wrap(w ...Inspector) Inspector {
	return func(f InspectorFunc) InspectorFunc {
		for i := len(w) - 1; i >= 0; i-- {
			f = w[i](f)
		}

		return f
	}
}
