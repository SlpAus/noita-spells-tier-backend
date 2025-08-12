package shutdown

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SlpAus/noita-spells-tier-backend/internal/platform/backup"
	"github.com/SlpAus/noita-spells-tier-backend/pkg/lifecycle"
)

const (
	httpTimeout     = 15 * time.Second
	gracefulTimeout = 30 * time.Second
	forcefulTimeout = 1 * time.Second
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

	// 第一步：关闭HTTP服务
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), httpTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		fmt.Printf("Gin服务器关闭错误: %v\n", err)
	} else {
		fmt.Println("Gin服务器已关闭。")
	}

	// 第二步：关闭第一阶段服务
	fmt.Println("开始关闭第一阶段服务（优雅停机）...")
	c.GracefulManager.Shutdown()
	remainingGraceful := c.GracefulManager.WaitWithTimeout(gracefulTimeout)
	if len(remainingGraceful) > 0 {
		fmt.Printf("警告：以下优雅停机服务未能按时退出: %v\n", remainingGraceful)
	}
	fmt.Println("第一阶段服务关闭完成.")

	// 第三步：关闭第二阶段服务
	fmt.Println("开始关闭第二阶段服务（强制停机）...")
	c.ForcefulManager.Shutdown()
	remainingForceful := c.ForcefulManager.WaitWithTimeout(forcefulTimeout)
	if len(remainingForceful) > 0 {
		fmt.Printf("警告：以下强制停机服务未能按时退出: %v\n", remainingForceful)
	}
	fmt.Println("第二阶段服务关闭完成.")

	// 第四步：创建最终数据快照
	fmt.Println("正在执行停机时快照...")
	if err := backup.CreateConsistentSnapshotInDB(context.Background()); err != nil {
		fmt.Printf("停机时快照失败: %v\n", err)
	} else {
		fmt.Println("停机时快照成功。")
	}

	fmt.Println("优雅停机完成。")
}
