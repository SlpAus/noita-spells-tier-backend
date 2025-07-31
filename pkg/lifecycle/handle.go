package lifecycle

import (
	"context"
	"time"
)

// Handle 是分发给每个后台服务的生命周期控制器。
// 它由 Manager 创建，并封装了服务的关闭逻辑。
type Handle struct {
	ctx context.Context
	// Close 是一个函数，用于通知Manager其所属的服务已经完成关闭。
	// 它应该在服务的Goroutine退出前通过 defer 来调用。
	Close func()
}

// Ctx 返回Handle内部的ctx
func (h *Handle) Ctx() context.Context {
	return h.ctx
}

// Done 返回一个channel，当生命周期管理器发出停机信号时，该channel会关闭。
// 它允许服务在select语句中监听停机信号。
func (h *Handle) Done() <-chan struct{} {
	return h.ctx.Done()
}

// Err 在Done()的channel关闭后，返回上下文被取消的原因。
func (h *Handle) Err() error {
	return h.ctx.Err()
}

// Sleep 暂停指定的时长，但如果生命周期句柄被取消，则会提前返回错误。
// 这是所有后台重试循环中推荐使用的休眠方法。
func (h *Handle) Sleep(duration time.Duration) error {
	// 创建一个定时器，用于正常的休眠
	timer := time.NewTimer(duration)

	select {
	case <-h.Done():
		// 确保定时器资源在函数退出时被清理
		if !timer.Stop() {
			<-timer.C
		}
		// 如果在休眠期间，上下文被取消，则立刻返回上下文的错误。
		return h.Err()
	case <-timer.C:
		// 如果定时器正常结束，则返回nil，表示休眠完成。
		return nil
	}
}
