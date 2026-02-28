package events

import (
	"context"
	"sync"
	"time"
)

var (
	defaultBus   *Bus
	defaultBusMu sync.RWMutex
)

// DefaultBus 返回进程内默认事件总线；若未设置则返回 nil，调用方需做空指针判断。
func DefaultBus() *Bus {
	defaultBusMu.RLock()
	defer defaultBusMu.RUnlock()
	return defaultBus
}

// SetDefaultBus 设置默认事件总线（常用于测试或主程序初始化)。
func SetDefaultBus(b *Bus) {
	defaultBusMu.Lock()
	defer defaultBusMu.Unlock()
	defaultBus = b
}

// Bus 用于发布与订阅生命周期事件，支持同步与异步监听器。
type Bus struct {
	mu        sync.RWMutex
	syncSubs  []Listener
	asyncSubs []Listener
}

// NewBus 创建一个新的事件总线。
func NewBus() *Bus {
	return &Bus{}
}

// Subscribe 注册监听器。async 为 true 时在独立 goroutine 中调用，不阻塞发布方。
func (b *Bus) Subscribe(async bool, l Listener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if async {
		b.asyncSubs = append(b.asyncSubs, l)
	} else {
		b.syncSubs = append(b.syncSubs, l)
	}
}

// Publish 向所有监听器发布事件。先执行完所有同步监听器，再启动异步监听器。
func (b *Bus) Publish(ctx context.Context, e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	b.mu.RLock()
	syncCopy := make([]Listener, len(b.syncSubs))
	copy(syncCopy, b.syncSubs)
	asyncCopy := make([]Listener, len(b.asyncSubs))
	copy(asyncCopy, b.asyncSubs)
	b.mu.RUnlock()

	for _, l := range syncCopy {
		l(ctx, e)
	}
	for _, l := range asyncCopy {
		go l(ctx, e)
	}
}
