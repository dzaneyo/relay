# Relay

Relay 是一个本地优先的 Host、Database、Note 管理与 SSH 连接工具。它提供 Web 界面和命令行入口，用来集中管理连接信息、跳板路线和工作笔记。

## 当前能力

- **Web UI**：管理 HOST、DATABASE、NOTE 记录，支持搜索、收藏、编辑和删除。
- **CLI**：支持 `list`、`search`、`show`、`connect` 和 `web`。
- **HOST 路由**：HOST 可以直连，也可以配置经过多个 HOST 跳板的连接路线；`connect` 会解析并展示路线计划。SSH 会话的实际启动尚未启用。
- **DATABASE**：支持 MySQL 和 PostgreSQL 连接信息，并可通过 CLI 调用对应客户端。
- **NOTE**：仅在 Web UI 中管理，不通过 CLI 的记录列表和搜索展示。

## 快速开始

项目使用 mise 管理 Go、Node.js 和 pnpm。安装并启用 mise 后，在项目根目录执行：

```bash
mise run setup
mise run build
mise run run
```

然后打开 <http://127.0.0.1:17321>。

开发时也可以分别启动后端和前端：

```bash
# 终端 1
mise run dev-backend

# 终端 2
mise run dev-frontend
```

开发页面地址为 <http://127.0.0.1:5173>。默认数据保存在 `~/.relay`，也可以通过 `RELAY_HOME` 指定目录。
