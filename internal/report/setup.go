package report

import (
	"github.com/SlpAus/noita-spells-tier-backend/internal/user"
)

// ClearMirrorRepo 在写锁的保护下，安全地重置内存仓库。
// 这通常在Redis恢复健康、缓存重建之后被调用，以确保下次降级时能加载最新的快照。
func ClearMirrorRepo() {
	mirrorRepo.mu.Lock()
	defer mirrorRepo.mu.Unlock()

	mirrorRepo.isLoaded = false
	mirrorRepo.userStats = nil
	mirrorRepo.userRank = nil
	mirrorRepo.totalStats = user.UserStats{}
	mirrorRepo.totalVoters = 0
}
