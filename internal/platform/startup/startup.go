package startup

import (
	"fmt"

	"github.com/SlpAus/noita-spells-tier-backend/internal/spell"
	"github.com/SlpAus/noita-spells-tier-backend/internal/vote"
	// "github.com/SlpAus/noita-spells-tier-backend/internal/user" // for future use
)

// InitializeApplication 是应用启动时执行的总入口
func InitializeApplication(flushCache bool) {
	fmt.Println("开始应用初始化...")

	// TODO: 在这里实现从metadata表检查上次是否正常退出的逻辑

	// 调用各个模块自己的初始化函数
	if err := spell.PrimeCachedDB(); err != nil {
		panic(err)
	}

	if err := vote.PrimeDB(); err != nil {
		panic(err)
	}

	// if err := user.WarmupCache(); err != nil { ... } // 未来扩展

	fmt.Println("应用初始化完成！")
}
