# Noita法术投票箱

一个基于社区投票的《Noita》法术排名系统。

现已初步拓展以支持法术投票与天赋投票双模式运行。

**后端技术栈**: Go, Gin, Redis, SQLite

**前端项目**: [SlpAus/noita-spells-tier-frontend](https://github.com/SlpAus/noita-spells-tier-frontend)

## 快速开始

### 依赖环境

* **Go** (版本 1.24+)
* **Docker**
* **LuaJIT**

---

### 一次性设置

在首次运行项目前，需要执行以下一次性的设置步骤。

1. **准备游戏资源**

* `[Noita]/data/translations/common.csv` -> `assets/data/translations/common.csv`
* `[Nolla_Games_Noita]/data/gun/gun_enums.lua` -> `assets/data/gun/gun_enums.lua`
* `[Nolla_Games_Noita]/data/scripts/gun/gun_actions.lua` -> `assets/data/scripts/gun/gun_actions.lua`
* `[Nolla_Games_Noita]/data/ui_gfx/gun_actions/` -> `assets/data/ui_gfx/gun_actions/`
* `[Nolla_Games_Noita]/data/scripts/perks/perks_list.lua` -> `assets/data/scripts/perks/perks_list.lua`
* `[Nolla_Games_Noita]/data/items_gfx/perks/` -> `assets/data/items_gfx/perks/`

2. **提取游戏数据**:

```bash
luajit ./build/lua_scripts/actions_extractor.lua
luajit ./build/lua_scripts/perks_extractor.lua
```

3. **构建初始数据库**:

```bash
CONFIG_NAME=config_spell go run ./build/go_scripts/build_database.go -task=build
CONFIG_NAME=config_perk go run ./build/go_scripts/build_database.go -task=build
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
CONFIG_NAME=config_spell go run ./cmd/server/main.go
CONFIG_NAME=config_perk go run ./cmd/server/main.go
```

---

### 配置

应用的核心配置位于 `config/config_spell.yaml`/`config/config_perk.yaml` 两个文件，分别对应法术和天赋两个模式下的后端。

* **`server`**: Gin服务器设置，包括运行模式 (`debug`/`release`)、监听地址和CORS跨域设置。`release`模式下Go部分不再路由`/images/spells`和`/images/perks`，这部分职责转交Nginx。
* **`app`**: 应用模式设置，包括法术模式 (`spell`)、天赋模式 (`perk`)。
* **`database`**: Redis连接信息，SQLite数据库文件名及缓存大小。

在部署或修改环境时，请相应地更新这些文件。
