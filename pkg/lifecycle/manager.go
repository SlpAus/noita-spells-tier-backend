package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 是一个功能完备的生命周期协调器。
// 它由一个上层模块（如shutdown）创建和持有，并向各个后台服务分发句柄(Handle)。
type Manager struct {
	wg       sync.WaitGroup
	mu       sync.Mutex
	services map[string]bool

	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager 创建一个新的、功能完備的生命周期管理器。
func NewManager() *Manager {
	m := &Manager{
		services: make(map[string]bool),
	}

	// 就地构造全新的Context
	m.ctx, m.cancel = context.WithCancel(context.Background())

	return m
}

// NewServiceHandle 为一个服务创建一个新的生命周期句柄(Handle)。
// 管理器会自动为这个服务注册并增加WaitGroup计数。
func (m *Manager) NewServiceHandle(name string) (*Handle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.services[name] {
		return nil, fmt.Errorf("生命周期管理器: 服务 '%s' 已被注册", name)
	}
	m.services[name] = true
	m.wg.Add(1)
	fmt.Printf("生命周期管理器: 服务 [%s] 已注册。\n", name)

	return &Handle{
		ctx: m.ctx,
		Close: func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if _, exists := m.services[name]; !exists {
				return
			}
			delete(m.services, name)
			m.wg.Done()
		},
	}, nil
}

func (m *Manager) Shutdown() {
	fmt.Println("生命周期管理器: 广播停机信号...")
	m.cancel()
}

// WaitWithTimeout 等待所有已注册的服务完成，直到指定的超时。
func (m *Manager) WaitWithTimeout(timeout time.Duration) []string {
	doneChan := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(doneChan)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-doneChan:
		return nil
	case <-timer.C:
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.getRemainingServices()
	}
}

// getRemainingServices 是一个内部辅助函数。
func (m *Manager) getRemainingServices() []string {
	remaining := make([]string, 0, len(m.services))
	for name := range m.services {
		remaining = append(remaining, name)
	}
	return remaining
}
