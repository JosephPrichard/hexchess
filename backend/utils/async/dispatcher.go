package async

import (
	"log/slog"
	"runtime/debug"
)

type Dispatcher interface {
	Go(func())
}

type SyncDispatcher struct{}

func (_ SyncDispatcher) Go(f func()) {
	f()
}

type AsyncDispatcher struct{}

func (_ AsyncDispatcher) Go(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic: fire and forget operation", "err", r, "stack", string(debug.Stack()))
			}
		}()
		f()
	}()
}
