package shutdown

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
)

// Coordinator 负责编排应用程序的优雅停机流程。
// 它接收外部创建的生命周期管理器，并使用它们来协调停机。
type Coordinator struct {
	GracefulManager *lifecycle.Manager
	ForcefulManager *lifecycle.Manager
}

// NewCoordinator 创建一个新的停机协调器。
func NewCoordinator(gracefulMgr, forcefulMgr *lifecycle.Manager) *Coordinator {
	return &Coordinator{
		GracefulManager: gracefulMgr,
		ForcefulManager: forcefulMgr,
	}
}

// ListenForSignalsAndShutdown 启动信号监听并阻塞，直到停机流程完成。
func (c *Coordinator) ListenForSignalsAndShutdown(server *http.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 阻塞直到接收到停机信号
	<-sigChan
	fmt.Println("\n收到关闭信号，开始优雅停机...")

	// 关闭HTTP服务器，允许正在进行的请求完成
	httpTimeout := 15 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), httpTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Gin服务器关闭错误: %v\n", err)
	} else {
		fmt.Println("Gin服务器已关闭。")
	}

	// --- 阶段一: 优雅停机 ---
	gracefulTimeout := 30 * time.Second
	fmt.Printf("第一阶段停机：等待最多 %v 以完成任务...\n", gracefulTimeout)
	// 广播第一阶段停机信号
	c.GracefulManager.Shutdown()

	// 等待所有后台服务完成
	remainingServices := c.GracefulManager.WaitWithTimeout(gracefulTimeout)
	if len(remainingServices) == 0 {
		fmt.Println("所有服务已在第一阶段优雅关闭。")
	} else {
		// --- 阶段二: 强制停机 ---
		forcefulTimeout := 1 * time.Second
		fmt.Printf("第一阶段超时。发送第二停机信号，强制退出 (最多等待 %v)...\n", forcefulTimeout)
		// 广播第二阶段停机信号
		c.ForcefulManager.Shutdown()
		// 在这里，我们不再等待，因为强制信号意味着“立即停止，不要再执行任何操作”
		// 服务的循环应该在接收到强制信号后立刻退出
		c.ForcefulManager.WaitWithTimeout(forcefulTimeout)
	}

	// --- 最终步骤 ---
	fmt.Println("正在执行最终快照...")
	if err := spell.CreateConsistentSnapshotInDB(context.Background()); err != nil {
		fmt.Printf("最终快照失败: %v\n", err)
	} else {
		fmt.Println("最终快照成功。")
	}

	fmt.Println("优雅停机完成。")
}
