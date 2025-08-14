# Noita法术投票箱

一个基于社区投票的《Noita》法术排名系统。

**后端技术栈**: Go, Gin, Redis, SQLite

## 快速开始

### 依赖环境

* **Go** (版本 1.24+)
* **Docker**
* **LuaJIT**

---

### 一次性设置

在首次运行项目前，需要执行以下一次性的设置步骤。

1. **准备游戏资源**

* `[Nolla_Games_Noita]/data/gun/gun_actions.lua` -> `assets/data/gun/gun_actions.lua`
* `[Nolla_Games_Noita]/data/gun/gun_enums.lua` -> `assets/data/gun/gun_enums.lua`
* `[Noita]/data/translations/common.csv` -> `assets/data/translations/common.csv`
* `[Nolla_Games_Noita]/data/ui_gfx/gun_actions/` -> `assets/data/ui_gfx/gun_actions/`

2. **提取法术数据**:

```bash
luajit ./build/lua_scripts/actions_extractor.lua
```

3. **构建初始数据库**:

```bash
go run ./build/go_scripts/build_database.go -task=build
```

4. **构建自定义Redis镜像**:

```bash
docker build -t my-redis:1.0 ./build/redis
```

---

### 运行应用

1. **启动Redis服务**:

```bash
docker run -d --name my-redis-instance -p 127.0.0.1:6379:6379 my-redis:1.0
```

2. **启动Go后端服务**:

```bash
go run ./cmd/server/main.go
```
