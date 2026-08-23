# relay Go + Vue3 开发手册

> 目标：使用 **Go + mise + Vue3 + SQLite** 实现一个本地优先的账号与连接信息管理工具。
>
> 第一阶段重点：
>
> - 保存普通账号/密码；
> - 保存 HOST 主机连接信息；
> - HOST 支持密码、SSH Key；
> - HOST 支持单跳/多跳跳板机；
> - 保存 DATABASE 连接信息；
> - CLI 支持按 alias 查询、查看和连接；
> - DATABASE 可通过本地终端客户端直接连接；
> - SSH 的真正启动逻辑先预留，第一阶段只完成配置解析与 SSH Command Plan；
> - 提供一个简单的 Vue3 管理页面；
> - 最终构建成一个 `relay` 单二进制文件，并将前端静态文件嵌入其中。
>
> 本文不讨论旧项目迁移。

---

## 1. 产品目标

relay 是一个本地工具，而不是一个传统后台系统。

期望最终体验：

```bash
# 查看全部记录
relay list

# 搜索
relay search edm

# 查看详情
relay show n33edm

# 连接数据库
relay connect mysql-test

# HOST 第一阶段先输出解析后的 SSH 计划
relay connect n33edm

# 启动本地 Web 管理页面
relay web
```

数据全部保存在本机 SQLite：

```text
~/.relay/
├── relay.db
└── logs/
```

第一版不引入服务端、不引入用户体系、不引入 Redis、不引入消息队列。

---

## 2. 第一阶段范围

### 2.1 支持的记录类型

```text
ACCOUNT
HOST
DATABASE
```

### 2.2 ACCOUNT

用于保存普通账号信息，例如：

```text
名称：GitLab Test
Alias：gitlab-test
Username：otis
Password：******
备注：测试环境 GitLab
```

### 2.3 HOST

例如：

```text
名称：EDM Node 33
Alias：n33edm
Host：172.17.113.33
Port：22
Username：otis
Auth：SSH_KEY
Key Path：~/.ssh/id_ed25519
Route：edm-prod-route
```

认证方式：

```text
NONE
PASSWORD
SSH_KEY
```

SSH Key 第一版只保存 **本机私钥路径**，不把私钥内容写入 SQLite。

### 2.4 DATABASE

第一版建议先支持：

```text
MYSQL
POSTGRESQL
```

后续再增加：

```text
ORACLE
SQLSERVER
CLICKHOUSE
DORIS
```

示例：

```text
名称：指标平台测试 MySQL
Alias：index-mysql-test
DB Type：MYSQL
Host：172.17.132.226
Port：3306
Database：index_process
Username：index_user
Password：******
```

### 2.5 SSH 跳板机

跳板机必须建模为 HOST 记录，不保存为裸字符串。

例如：

```text
HOST: bastion-a
  10.0.0.10:22
  user=otis
  SSH_KEY=~/.ssh/id_ed25519

HOST: bastion-b
  10.0.1.10:22
  user=otis
  SSH_KEY=~/.ssh/id_ed25519

HOST: n33edm
  172.17.113.33:22
  user=otis
  route=prod-route

ROUTE: prod-route
  hop 1 -> bastion-a
  hop 2 -> bastion-b
```

将来真正启动时可生成：

```bash
ssh \
  -J otis@10.0.0.10:22,otis@10.0.1.10:22 \
  -i ~/.ssh/id_ed25519 \
  -p 22 \
  otis@172.17.113.33
```

> 注意：OpenSSH 的 `-J` 对每一跳使用各跳自身 SSH 配置时存在更复杂的认证场景。第一阶段先把路由结构和 Command Plan 做正确；需要每跳不同 Key 时，后续生成临时 SSH config 会比继续拼 `-J` 参数更可靠。

---

# 3. 技术选型

## 3.1 后端

```text
Go 1.26
Cobra
Chi
SQLite
modernc.org/sqlite
log/slog
embed
```

依赖原则：尽量少。

不使用：

```text
Gin
GORM
Wire
Fx
大型配置框架
```

原因是 relay 的业务并不复杂。

## 3.2 前端

```text
Vue 3
TypeScript
Vite
原生 fetch
```

第一版不使用：

```text
Pinia
Vue Router
Element Plus
Ant Design Vue
```

页面控制在一个 SPA 中即可。

## 3.3 构建环境

统一由 mise 管理：

```text
Go
Node.js
pnpm
```

---

# 4. 总体架构

```text
                    relay
                      │
           ┌──────────┴──────────┐
           │                     │
          CLI                  relay web
           │                     │
           │                 HTTP Server
           │                     │
           └──────────┬──────────┘
                      │
                 App Service
                      │
       ┌──────────────┼──────────────┐
       │              │              │
     Record          SSH             DB
       │              │              │
       └──────────────┼──────────────┘
                      │
                  Repository
                      │
                    SQLite
```

前端：

```text
Vue3
  │
  │ /api/*
  ▼
Go HTTP Server
```

发布时：

```text
Vue dist
   │
   │ go:embed
   ▼
relay binary
```

最终只有：

```text
relay
```

---

# 5. 项目目录

最终建议目录：

```text
relay/
├── cmd/
│   └── relay/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   └── app.go
│   │
│   ├── cli/
│   │   ├── root.go
│   │   ├── list.go
│   │   ├── search.go
│   │   ├── show.go
│   │   ├── connect.go
│   │   └── web.go
│   │
│   ├── model/
│   │   └── model.go
│   │
│   ├── repository/
│   │   ├── record.go
│   │   ├── credential.go
│   │   ├── ssh.go
│   │   ├── route.go
│   │   └── database.go
│   │
│   ├── service/
│   │   ├── record.go
│   │   ├── connect.go
│   │   └── ssh_plan.go
│   │
│   ├── storage/
│   │   ├── sqlite.go
│   │   └── schema.sql
│   │
│   └── web/
│       ├── server.go
│       └── dist/
│           └── placeholder.txt
│
├── frontend/
│   ├── src/
│   │   ├── App.vue
│   │   ├── api.ts
│   │   ├── types.ts
│   │   └── main.ts
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
│
├── build/
├── mise.toml
├── go.mod
├── go.sum
├── .gitignore
└── README.md
```

这里没有按照传统 Java 项目拆大量 `controller/service/manager/domain/dao` 层。

保持：

```text
CLI / HTTP
   ↓
Service
   ↓
Repository
   ↓
SQLite
```

就够了。

---

# 6. 初始化开发环境

## 6.1 安装 mise

macOS：

```bash
brew install mise
```

Shell 初始化，例如 zsh：

```bash
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc
source ~/.zshrc
```

确认：

```bash
mise --version
```

---

# 7. 创建项目

```bash
mkdir relay
cd relay

git init
```

创建基础目录：

```bash
mkdir -p cmd/relay
mkdir -p internal/{app,cli,model,repository,service,storage,web/dist}
mkdir -p frontend
mkdir -p build

touch internal/web/dist/placeholder.txt
```

---

# 8. mise 配置

根目录创建：

```text
mise.toml
```

内容：

```toml
[tools]
go = "1.26"
node = "24"
pnpm = "10"

[tasks.setup]
description = "Install backend and frontend dependencies"
run = """
go mod download
cd frontend
pnpm install
"""

[tasks.dev-backend]
description = "Run relay backend"
run = "go run ./cmd/relay web"

[tasks.dev-frontend]
description = "Run Vue development server"
run = "cd frontend && pnpm dev"

[tasks.test]
description = "Run Go tests"
run = "go test ./..."

[tasks.build-frontend]
description = "Build Vue frontend into Go embed directory"
run = "cd frontend && pnpm build"

[tasks.build]
description = "Build relay single binary"
run = """
cd frontend
pnpm build
cd ..
mkdir -p build
go build -trimpath -ldflags='-s -w' -o build/relay ./cmd/relay
"""

[tasks.run]
description = "Run built binary"
run = "./build/relay"
```

安装工具：

```bash
mise install
```

确认：

```bash
go version
node --version
pnpm --version
```

---

# 9. 初始化 Go

假设 GitHub 模块名：

```text
github.com/yourname/relay
```

执行：

```bash
go mod init github.com/yourname/relay
```

安装依赖：

```bash
go get github.com/spf13/cobra

go get github.com/go-chi/chi/v5

go get github.com/google/uuid

go get modernc.org/sqlite
```

整理：

```bash
go mod tidy
```

---

# 10. 数据模型设计

## 10.1 表关系

```text
rd_records
   │
   ├────────────< rd_credentials
   │                   │
   │                   ├──────── rd_ssh_connections
   │                   │
   │                   └──────── rd_db_connections
   │
   └──────────── rd_ssh_connections
                         │
                         └──── route_id
                                │
                                ▼
                         rd_ssh_routes
                                │
                                ▼
                         rd_ssh_route_hops
                                │
                                ▼
                       HOST record_id
```

核心设计：

- `rd_records`：所有记录的公共信息；
- `rd_credentials`：用户名、密码或 SSH Key；
- `rd_ssh_connections`：HOST 的 SSH 地址；
- `rd_ssh_routes`：跳板路由；
- `rd_ssh_route_hops`：路由中按顺序引用 HOST；
- `rd_db_connections`：DATABASE 的连接参数。

这样既不复杂，也不会把 SSH 路由塞成 JSON。

---

# 11. SQLite Schema

创建：

```text
internal/storage/schema.sql
```

内容：

```sql
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS rd_records (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    alias       TEXT NOT NULL,
    category    TEXT NOT NULL,
    notes       TEXT NOT NULL DEFAULT '',
    favorite    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_records_alias
    ON rd_records(alias COLLATE NOCASE);

CREATE INDEX IF NOT EXISTS idx_rd_records_category
    ON rd_records(category);

CREATE TABLE IF NOT EXISTS rd_credentials (
    id           TEXT PRIMARY KEY,
    record_id    TEXT NOT NULL,
    label        TEXT NOT NULL DEFAULT 'default',
    username     TEXT,
    auth_type    TEXT NOT NULL,
    secret_value TEXT,
    key_path     TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    FOREIGN KEY(record_id)
        REFERENCES rd_records(id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rd_credentials_record
    ON rd_credentials(record_id);

CREATE TABLE IF NOT EXISTS rd_ssh_routes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_ssh_routes_name
    ON rd_ssh_routes(name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS rd_ssh_connections (
    record_id     TEXT PRIMARY KEY,
    host          TEXT NOT NULL,
    port          INTEGER NOT NULL DEFAULT 22,
    credential_id TEXT,
    route_id      TEXT,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    FOREIGN KEY(record_id)
        REFERENCES rd_records(id)
        ON DELETE CASCADE,
    FOREIGN KEY(credential_id)
        REFERENCES rd_credentials(id)
        ON DELETE SET NULL,
    FOREIGN KEY(route_id)
        REFERENCES rd_ssh_routes(id)
        ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS rd_ssh_route_hops (
    route_id       TEXT NOT NULL,
    seq            INTEGER NOT NULL,
    host_record_id TEXT NOT NULL,
    PRIMARY KEY(route_id, seq),
    FOREIGN KEY(route_id)
        REFERENCES rd_ssh_routes(id)
        ON DELETE CASCADE,
    FOREIGN KEY(host_record_id)
        REFERENCES rd_records(id)
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_rd_ssh_route_hops_host
    ON rd_ssh_route_hops(host_record_id);

CREATE TABLE IF NOT EXISTS rd_db_connections (
    record_id      TEXT PRIMARY KEY,
    db_type        TEXT NOT NULL,
    host           TEXT NOT NULL,
    port           INTEGER NOT NULL,
    database_name  TEXT,
    credential_id  TEXT,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL,
    FOREIGN KEY(record_id)
        REFERENCES rd_records(id)
        ON DELETE CASCADE,
    FOREIGN KEY(credential_id)
        REFERENCES rd_credentials(id)
        ON DELETE SET NULL
);
```

第一版密码是本地 SQLite 明文保存。

因此至少做到：

```text
~/.relay              0700
~/.relay/relay.db 0600
```

后面需要增强安全性时，再把 `secret_value` 替换为 macOS Keychain / Windows Credential Manager / Linux Secret Service 的引用，不需要推翻其他表。

---

# 12. SQLite 初始化代码

创建：

```text
internal/storage/sqlite.go
```

```go
package storage

import (
    "database/sql"
    "embed"
    "fmt"
    "os"
    "path/filepath"

    _ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

func DefaultHome() (string, error) {
    if home := os.Getenv("relay_HOME"); home != "" {
        return home, nil
    }

    userHome, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }

    return filepath.Join(userHome, ".relay"), nil
}

func Open() (*sql.DB, error) {
    home, err := DefaultHome()
    if err != nil {
        return nil, err
    }

    if err := os.MkdirAll(home, 0o700); err != nil {
        return nil, err
    }

    dbPath := filepath.Join(home, "relay.db")

    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }

    if _, err := db.Exec(`
        PRAGMA foreign_keys = ON;
        PRAGMA journal_mode = WAL;
        PRAGMA busy_timeout = 5000;
    `); err != nil {
        db.Close()
        return nil, fmt.Errorf("configure sqlite: %w", err)
    }

    schema, err := schemaFS.ReadFile("schema.sql")
    if err != nil {
        db.Close()
        return nil, err
    }

    if _, err := db.Exec(string(schema)); err != nil {
        db.Close()
        return nil, fmt.Errorf("initialize schema: %w", err)
    }

    // SQLite 文件在首次写入后才一定存在，因此 chmod 失败时仅忽略不存在场景。
    if _, err := os.Stat(dbPath); err == nil {
        _ = os.Chmod(dbPath, 0o600)
    }

    return db, nil
}
```

---

# 13. Go Model

创建：

```text
internal/model/model.go
```

```go
package model

type RecordCategory string

const (
    CategoryAccount  RecordCategory = "ACCOUNT"
    CategoryHost     RecordCategory = "HOST"
    CategoryDatabase RecordCategory = "DATABASE"
)

type AuthType string

const (
    AuthNone   AuthType = "NONE"
    AuthPass   AuthType = "PASSWORD"
    AuthSSHKey AuthType = "SSH_KEY"
)

type DBType string

const (
    DBMySQL      DBType = "MYSQL"
    DBPostgreSQL DBType = "POSTGRESQL"
)

type Record struct {
    ID        string         `json:"id"`
    Name      string         `json:"name"`
    Alias     string         `json:"alias"`
    Category  RecordCategory `json:"category"`
    Notes     string         `json:"notes"`
    Favorite  bool           `json:"favorite"`
    CreatedAt string         `json:"createdAt"`
    UpdatedAt string         `json:"updatedAt"`
}

type Credential struct {
    ID          string   `json:"id"`
    RecordID    string   `json:"recordId"`
    Label       string   `json:"label"`
    Username    string   `json:"username"`
    AuthType    AuthType `json:"authType"`
    SecretValue string   `json:"secretValue,omitempty"`
    KeyPath     string   `json:"keyPath,omitempty"`
    CreatedAt   string   `json:"createdAt"`
    UpdatedAt   string   `json:"updatedAt"`
}

type SSHConnection struct {
    RecordID     string `json:"recordId"`
    Host         string `json:"host"`
    Port         int    `json:"port"`
    CredentialID string `json:"credentialId,omitempty"`
    RouteID      string `json:"routeId,omitempty"`
    CreatedAt    string `json:"createdAt"`
    UpdatedAt    string `json:"updatedAt"`
}

type SSHRoute struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Description string     `json:"description"`
    Hops        []RouteHop `json:"hops"`
    CreatedAt   string     `json:"createdAt"`
    UpdatedAt   string     `json:"updatedAt"`
}

type RouteHop struct {
    Seq          int    `json:"seq"`
    HostRecordID string `json:"hostRecordId"`
}

type DBConnection struct {
    RecordID     string `json:"recordId"`
    DBType       DBType `json:"dbType"`
    Host         string `json:"host"`
    Port         int    `json:"port"`
    DatabaseName string `json:"databaseName"`
    CredentialID string `json:"credentialId,omitempty"`
    CreatedAt    string `json:"createdAt"`
    UpdatedAt    string `json:"updatedAt"`
}

type RecordDetail struct {
    Record     Record         `json:"record"`
    Credential *Credential    `json:"credential,omitempty"`
    SSH        *SSHConnection `json:"ssh,omitempty"`
    Database   *DBConnection  `json:"database,omitempty"`
}
```

---

# 14. Repository 编写原则

Repository 只做 SQL，不放业务判断。

例如：

```text
RecordRepository
CredentialRepository
SSHRepository
RouteRepository
DatabaseRepository
```

Service 决定：

```text
HOST 必须有 ssh connection
DATABASE 必须有 db connection
SSH_KEY 必须填写 key_path
PASSWORD 可以填写 secret_value
route hop 必须引用 HOST
route 不允许引用自己形成循环
```

---

# 15. Record Repository

创建：

```text
internal/repository/record.go
```

```go
package repository

import (
    "context"
    "database/sql"
    "strings"

    "github.com/yourname/relay/internal/model"
)

type RecordRepository struct {
    db *sql.DB
}

func NewRecordRepository(db *sql.DB) *RecordRepository {
    return &RecordRepository{db: db}
}

func (r *RecordRepository) List(ctx context.Context) ([]model.Record, error) {
    rows, err := r.db.QueryContext(ctx, `
        SELECT id, name, alias, category, notes, favorite, created_at, updated_at
        FROM rd_records
        ORDER BY favorite DESC, updated_at DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var result []model.Record
    for rows.Next() {
        var item model.Record
        var favorite int

        if err := rows.Scan(
            &item.ID,
            &item.Name,
            &item.Alias,
            &item.Category,
            &item.Notes,
            &favorite,
            &item.CreatedAt,
            &item.UpdatedAt,
        ); err != nil {
            return nil, err
        }

        item.Favorite = favorite != 0
        result = append(result, item)
    }

    return result, rows.Err()
}

func (r *RecordRepository) Search(ctx context.Context, keyword string) ([]model.Record, error) {
    keyword = "%" + strings.TrimSpace(keyword) + "%"

    rows, err := r.db.QueryContext(ctx, `
        SELECT id, name, alias, category, notes, favorite, created_at, updated_at
        FROM rd_records
        WHERE name LIKE ? COLLATE NOCASE
           OR alias LIKE ? COLLATE NOCASE
           OR notes LIKE ? COLLATE NOCASE
        ORDER BY favorite DESC, updated_at DESC
    `, keyword, keyword, keyword)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var result []model.Record
    for rows.Next() {
        var item model.Record
        var favorite int
        if err := rows.Scan(
            &item.ID,
            &item.Name,
            &item.Alias,
            &item.Category,
            &item.Notes,
            &favorite,
            &item.CreatedAt,
            &item.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        item.Favorite = favorite != 0
        result = append(result, item)
    }

    return result, rows.Err()
}

func (r *RecordRepository) FindByAlias(ctx context.Context, alias string) (*model.Record, error) {
    row := r.db.QueryRowContext(ctx, `
        SELECT id, name, alias, category, notes, favorite, created_at, updated_at
        FROM rd_records
        WHERE alias = ? COLLATE NOCASE
    `, alias)

    var item model.Record
    var favorite int

    if err := row.Scan(
        &item.ID,
        &item.Name,
        &item.Alias,
        &item.Category,
        &item.Notes,
        &favorite,
        &item.CreatedAt,
        &item.UpdatedAt,
    ); err != nil {
        return nil, err
    }

    item.Favorite = favorite != 0
    return &item, nil
}

func (r *RecordRepository) FindByID(ctx context.Context, id string) (*model.Record, error) {
    row := r.db.QueryRowContext(ctx, `
        SELECT id, name, alias, category, notes, favorite, created_at, updated_at
        FROM rd_records
        WHERE id = ?
    `, id)

    var item model.Record
    var favorite int

    if err := row.Scan(
        &item.ID,
        &item.Name,
        &item.Alias,
        &item.Category,
        &item.Notes,
        &favorite,
        &item.CreatedAt,
        &item.UpdatedAt,
    ); err != nil {
        return nil, err
    }

    item.Favorite = favorite != 0
    return &item, nil
}
```

创建/更新操作建议后续统一放在 `RecordService.Save()` 的事务里完成，避免 Web 保存 HOST 时出现主记录写成功而 SSH 配置失败。

---

# 16. App 容器

创建：

```text
internal/app/app.go
```

```go
package app

import (
    "database/sql"

    "github.com/yourname/relay/internal/repository"
)

type App struct {
    DB      *sql.DB
    Records *repository.RecordRepository
}

func New(db *sql.DB) *App {
    return &App{
        DB:      db,
        Records: repository.NewRecordRepository(db),
    }
}
```

后面增加：

```text
Credentials
SSH
Routes
Databases
RecordService
ConnectService
```

不需要 IOC 容器。

显式 `New()` 即可。

---

# 17. SSH Route 解析规则

这是整个 SSH 设计最重要的一块。

一条 route：

```text
route
  ├── hop 1 -> HOST A
  └── hop 2 -> HOST B
```

解析时：

1. 按 `seq` 查询 `rd_ssh_route_hops`；
2. 每个 `host_record_id` 必须是 `HOST`；
3. 读取 HOST 对应 `rd_ssh_connections`；
4. 读取它引用的 credential；
5. 组合成 `ResolvedHop`；
6. 最终组合 Target HOST。

建议模型：

```go
type ResolvedSSHNode struct {
    Alias      string
    Host       string
    Port       int
    Username   string
    AuthType   model.AuthType
    Secret     string
    KeyPath    string
}

type SSHPlan struct {
    Hops   []ResolvedSSHNode
    Target ResolvedSSHNode
}
```

第一阶段：

```text
relay connect n33edm
```

如果是 HOST，先输出：

```text
SSH connection is not enabled yet.

Route:
  1. bastion-a  otis@10.0.0.10:22  SSH_KEY ~/.ssh/id_ed25519
  2. bastion-b  otis@10.0.1.10:22  SSH_KEY ~/.ssh/id_ed25519
Target:
  n33edm      otis@172.17.113.33:22 SSH_KEY ~/.ssh/id_ed25519
```

这样先把最关键的配置和解析逻辑验证正确。

---

# 18. 后续 SSH 启动接口预留

创建：

```text
internal/service/ssh_plan.go
```

接口先这样：

```go
package service

import "context"

type SSHLauncher interface {
    Launch(ctx context.Context, plan SSHPlan) error
}
```

第一阶段提供：

```go
type DisabledSSHLauncher struct{}

func (DisabledSSHLauncher) Launch(ctx context.Context, plan SSHPlan) error {
    return ErrSSHLaunchNotEnabled
}
```

以后真正实现时再增加：

```text
SystemSSHLauncher
```

最终内部仍然建议调用系统 OpenSSH：

```go
exec.CommandContext(ctx, "ssh", args...)
```

不要自己实现 SSH 协议。

---

# 19. DATABASE 连接方式

第一阶段数据库终端连接走系统客户端。

例如：

```text
MYSQL      -> mysql
POSTGRESQL -> psql
```

先检查命令是否存在：

```go
path, err := exec.LookPath("mysql")
```

不存在就输出：

```text
mysql client not found in PATH
```

---

# 20. ConnectService

创建：

```text
internal/service/connect.go
```

核心结构：

```go
package service

import (
    "context"
    "errors"
    "fmt"
    "os"
    "os/exec"

    "github.com/yourname/relay/internal/model"
)

var ErrSSHLaunchNotEnabled = errors.New("ssh launch is not enabled yet")

type DetailLoader interface {
    FindDetailByAlias(ctx context.Context, alias string) (*model.RecordDetail, error)
}

type ConnectService struct {
    loader DetailLoader
}

func NewConnectService(loader DetailLoader) *ConnectService {
    return &ConnectService{loader: loader}
}

func (s *ConnectService) Connect(ctx context.Context, alias string) error {
    detail, err := s.loader.FindDetailByAlias(ctx, alias)
    if err != nil {
        return err
    }

    switch detail.Record.Category {
    case model.CategoryHost:
        return ErrSSHLaunchNotEnabled

    case model.CategoryDatabase:
        return s.connectDatabase(ctx, detail)

    default:
        return fmt.Errorf("record %q is not connectable", alias)
    }
}

func (s *ConnectService) connectDatabase(ctx context.Context, detail *model.RecordDetail) error {
    if detail.Database == nil {
        return errors.New("database configuration not found")
    }

    switch detail.Database.DBType {
    case model.DBMySQL:
        return s.connectMySQL(ctx, detail)
    case model.DBPostgreSQL:
        return s.connectPostgreSQL(ctx, detail)
    default:
        return fmt.Errorf("unsupported database type: %s", detail.Database.DBType)
    }
}

func (s *ConnectService) connectPostgreSQL(ctx context.Context, detail *model.RecordDetail) error {
    db := detail.Database

    args := []string{
        "-h", db.Host,
        "-p", fmt.Sprintf("%d", db.Port),
    }

    env := os.Environ()

    if detail.Credential != nil {
        if detail.Credential.Username != "" {
            args = append(args, "-U", detail.Credential.Username)
        }
        if detail.Credential.SecretValue != "" {
            env = append(env, "PGPASSWORD="+detail.Credential.SecretValue)
        }
    }

    if db.DatabaseName != "" {
        args = append(args, db.DatabaseName)
    }

    cmd := exec.CommandContext(ctx, "psql", args...)
    cmd.Env = env
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    return cmd.Run()
}
```

MySQL 不建议把密码放到命令行参数：

```bash
mysql --password=xxx
```

因为其他本机进程可能通过进程参数看到它。

第一版有两个选择：

### 简单方案

```bash
mysql -h host -P 3306 -u user -p database
```

由 mysql 自己提示密码。

### 自动登录方案

启动前创建 `0600` 临时 defaults 文件：

```text
[client]
host=172.17.132.226
port=3306
user=index_user
password=xxxx
```

然后：

```bash
mysql --defaults-extra-file=/tmp/relay-xxx.cnf index_process
```

进程结束后立即删除临时文件。

建议第二阶段再做自动 MySQL 密码注入；第一版先保证命令调用链稳定。

---

# 21. CLI

使用 Cobra。

CLI 结构：

```text
relay
├── list
├── search <keyword>
├── show <alias>
├── connect <alias>
└── web
```

另外支持快捷方式：

```bash
relay n33edm
```

等价于：

```bash
relay connect n33edm
```

快捷方式建议等基础 CLI 稳定后再加。

---

# 22. main.go

创建：

```text
cmd/relay/main.go
```

```go
package main

import (
    "fmt"
    "os"

    "github.com/yourname/relay/internal/app"
    "github.com/yourname/relay/internal/cli"
    "github.com/yourname/relay/internal/storage"
)

func main() {
    db, err := storage.Open()
    if err != nil {
        fmt.Fprintln(os.Stderr, "open database:", err)
        os.Exit(1)
    }
    defer db.Close()

    application := app.New(db)

    root := cli.NewRootCommand(application)
    if err := root.Execute(); err != nil {
        os.Exit(1)
    }
}
```

---

# 23. Root Command

创建：

```text
internal/cli/root.go
```

```go
package cli

import (
    "github.com/spf13/cobra"

    "github.com/yourname/relay/internal/app"
)

func NewRootCommand(application *app.App) *cobra.Command {
    root := &cobra.Command{
        Use:   "relay",
        Short: "Local account and connection manager",
    }

    root.AddCommand(newListCommand(application))
    root.AddCommand(newSearchCommand(application))
    root.AddCommand(newShowCommand(application))
    root.AddCommand(newConnectCommand(application))
    root.AddCommand(newWebCommand(application))

    return root
}
```

---

# 24. list 命令

```text
internal/cli/list.go
```

```go
package cli

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/yourname/relay/internal/app"
)

func newListCommand(application *app.App) *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List records",
        RunE: func(cmd *cobra.Command, args []string) error {
            records, err := application.Records.List(cmd.Context())
            if err != nil {
                return err
            }

            for _, item := range records {
                fmt.Printf("%-20s %-10s %s\n", item.Alias, item.Category, item.Name)
            }

            return nil
        },
    }
}
```

效果：

```text
n33edm               HOST       EDM Node 33
index-mysql-test     DATABASE   指标平台测试 MySQL
gitlab-test          ACCOUNT    GitLab Test
```

---

# 25. search 命令

```text
internal/cli/search.go
```

核心：

```go
records, err := application.Records.Search(cmd.Context(), args[0])
```

输出格式保持和 `list` 一致。

不要第一版就引入全文检索引擎。

SQLite LIKE 足够。

---

# 26. show 命令

```bash
relay show n33edm
```

输出建议：

```text
Name:       EDM Node 33
Alias:      n33edm
Category:   HOST
Host:       172.17.113.33
Port:       22
Username:   otis
Auth:       SSH_KEY
Key Path:   ~/.ssh/id_ed25519
Route:      prod-route
```

密码默认：

```text
Password:   ******
```

不要默认在终端直接输出明文密码。

以后需要：

```bash
relay show gitlab-test --reveal
```

再显式显示。

---

# 27. Web API

HTTP 只监听：

```text
127.0.0.1:17321
```

不要默认监听：

```text
0.0.0.0
```

第一版 API：

| Method | Path | 作用 |
|---|---|---|
| GET | `/api/records` | 查询记录 |
| GET | `/api/records/{id}` | 详情 |
| POST | `/api/records` | 创建 |
| PUT | `/api/records/{id}` | 更新 |
| DELETE | `/api/records/{id}` | 删除 |
| GET | `/api/routes` | SSH Route 列表 |
| POST | `/api/routes` | 创建 Route |
| PUT | `/api/routes/{id}` | 更新 Route |
| DELETE | `/api/routes/{id}` | 删除 Route |
| GET | `/api/health` | 健康检查 |

建议记录创建/更新使用一个聚合请求，而不是前端调用三四个接口。

例如 HOST：

```json
{
  "name": "EDM Node 33",
  "alias": "n33edm",
  "category": "HOST",
  "notes": "",
  "credential": {
    "username": "otis",
    "authType": "SSH_KEY",
    "keyPath": "~/.ssh/id_ed25519"
  },
  "ssh": {
    "host": "172.17.113.33",
    "port": 22,
    "routeId": "route-xxx"
  }
}
```

DATABASE：

```json
{
  "name": "Index MySQL Test",
  "alias": "index-mysql-test",
  "category": "DATABASE",
  "credential": {
    "username": "index_user",
    "authType": "PASSWORD",
    "secretValue": "xxxx"
  },
  "database": {
    "dbType": "MYSQL",
    "host": "172.17.132.226",
    "port": 3306,
    "databaseName": "index_process"
  }
}
```

---

# 28. Web Server

创建：

```text
internal/web/server.go
```

基础版本：

```go
package web

import (
    "embed"
    "encoding/json"
    "io/fs"
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/yourname/relay/internal/app"
)

//go:embed all:dist
var frontendFS embed.FS

type Server struct {
    app *app.App
}

func NewServer(application *app.App) *Server {
    return &Server{app: application}
}

func (s *Server) Handler() http.Handler {
    r := chi.NewRouter()

    r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
    })

    r.Get("/api/records", s.listRecords)

    staticFS, _ := fs.Sub(frontendFS, "dist")
    fileServer := http.FileServer(http.FS(staticFS))

    r.Handle("/*", fileServer)

    return r
}

func (s *Server) listRecords(w http.ResponseWriter, r *http.Request) {
    records, err := s.app.Records.List(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    writeJSON(w, http.StatusOK, records)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(value)
}
```

第一版 Vue 不使用 Router，因此静态文件处理非常简单。

---

# 29. web CLI 命令

```text
internal/cli/web.go
```

```go
package cli

import (
    "fmt"
    "net/http"

    "github.com/spf13/cobra"

    "github.com/yourname/relay/internal/app"
    webserver "github.com/yourname/relay/internal/web"
)

func newWebCommand(application *app.App) *cobra.Command {
    return &cobra.Command{
        Use:   "web",
        Short: "Start local web UI",
        RunE: func(cmd *cobra.Command, args []string) error {
            server := webserver.NewServer(application)
            addr := "127.0.0.1:17321"

            fmt.Println("relay Web: http://" + addr)
            return http.ListenAndServe(addr, server.Handler())
        },
    }
}
```

运行：

```bash
mise run dev-backend
```

访问：

```text
http://127.0.0.1:17321/api/health
```

---

# 30. 初始化 Vue3

在根目录执行：

```bash
pnpm create vite frontend --template vue-ts
```

如果 `frontend` 已创建，先删除空目录再执行。

然后：

```bash
cd frontend
pnpm install
cd ..
```

---

# 31. Vite 配置

修改：

```text
frontend/vite.config.ts
```

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [vue()],

  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:17321',
        changeOrigin: true,
      },
    },
  },

  build: {
    outDir: resolve(__dirname, '../internal/web/dist'),
    emptyOutDir: true,
  },
})
```

开发阶段：

```text
5173       Vue
17321      Go API
```

Vite 自动代理 `/api`。

发布阶段 Vue 编译到：

```text
internal/web/dist
```

然后被 Go `embed` 进二进制。

---

# 32. 前端类型

创建：

```text
frontend/src/types.ts
```

```ts
export type RecordCategory = 'ACCOUNT' | 'HOST' | 'DATABASE'
export type AuthType = 'NONE' | 'PASSWORD' | 'SSH_KEY'
export type DBType = 'MYSQL' | 'POSTGRESQL'

export interface RecordSummary {
  id: string
  name: string
  alias: string
  category: RecordCategory
  notes: string
  favorite: boolean
  createdAt: string
  updatedAt: string
}

export interface CredentialForm {
  username: string
  authType: AuthType
  secretValue: string
  keyPath: string
}

export interface SSHForm {
  host: string
  port: number
  routeId: string
}

export interface DatabaseForm {
  dbType: DBType
  host: string
  port: number
  databaseName: string
}
```

---

# 33. 前端 API

创建：

```text
frontend/src/api.ts
```

```ts
import type { RecordSummary } from './types'

export async function listRecords(): Promise<RecordSummary[]> {
  const response = await fetch('/api/records')

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  return response.json()
}
```

以后统一扩展：

```text
getRecord()
createRecord()
updateRecord()
deleteRecord()
listRoutes()
createRoute()
updateRoute()
```

第一版不需要 Axios。

---

# 34. 简单 App.vue

第一版页面建议：

```text
┌──────────────────────────────────────────┐
│ relay                    + New       │
├───────────────┬──────────────────────────┤
│ Search        │ Record Detail            │
│               │                          │
│ HOST          │ Name                     │
│ n33edm        │ Alias                    │
│               │ Type                     │
│ DATABASE      │ ...                      │
│ index-mysql   │                          │
│               │                          │
│ ACCOUNT       │                 Save     │
│ gitlab-test   │                          │
└───────────────┴──────────────────────────┘
```

不要第一版做复杂 Dashboard。

最基础 `App.vue` 可以先从列表开始：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { listRecords } from './api'
import type { RecordSummary } from './types'

const records = ref<RecordSummary[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''

  try {
    records.value = await listRecords()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <main class="page">
    <header class="header">
      <div>
        <h1>relay</h1>
        <p>Local accounts and connections</p>
      </div>
      <button>New</button>
    </header>

    <p v-if="loading">Loading...</p>
    <p v-if="error" class="error">{{ error }}</p>

    <section class="records">
      <article v-for="record in records" :key="record.id" class="record-card">
        <div class="meta">{{ record.category }}</div>
        <strong>{{ record.name }}</strong>
        <code>{{ record.alias }}</code>
      </article>
    </section>
  </main>
</template>

<style scoped>
.page {
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 20px;
  font-family: Inter, ui-sans-serif, system-ui, sans-serif;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.header h1 {
  margin: 0;
}

.header p {
  margin: 6px 0 0;
  color: #666;
}

.records {
  display: grid;
  gap: 12px;
}

.record-card {
  display: grid;
  grid-template-columns: 120px 1fr 180px;
  align-items: center;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 10px;
}

.meta {
  font-size: 12px;
  color: #666;
}

.error {
  color: #b00020;
}
</style>
```

接下来再补编辑表单。

不要一开始就把所有 UI 一次写完。

---

# 35. HOST 编辑页面字段

HOST 表单固定字段：

```text
Name
Alias
Notes

Host
Port
Username
Auth Type
Password / Key Path
SSH Route
```

动态规则：

```text
AuthType = NONE
  不展示 Secret

AuthType = PASSWORD
  展示 Password

AuthType = SSH_KEY
  展示 Key Path
```

Route 下拉框：

```text
None
prod-route
staging-route
```

---

# 36. SSH Route 页面

第一版可以做一个非常简单的 Route 管理页：

```text
Route Name: prod-route
Description: production bastions

Hops:
1  bastion-a   ↑ ↓ Delete
2  bastion-b   ↑ ↓ Delete

+ Add Hop

Save
```

`Add Hop` 只能选择：

```text
category = HOST
```

并建议校验：

```text
跳板 HOST 自己不能再配置 route
```

第一版禁止嵌套路由，可以大幅减少循环依赖和排错复杂度。

也就是说：

```text
目标 HOST -> route -> HOST A -> HOST B
```

允许。

但：

```text
HOST A -> 另外一个 route
```

第一版禁止。

---

# 37. DATABASE 编辑字段

```text
Name
Alias
Notes

DB Type
Host
Port
Database Name
Username
Password
```

端口默认值：

```text
MYSQL       3306
POSTGRESQL  5432
```

前端切换 DB Type 时，如果用户还没修改 port，可以自动更新默认端口。

---

# 38. ACCOUNT 编辑字段

```text
Name
Alias
Notes
Username
Password
```

第一版一条 ACCOUNT 对应一组 credential。

如果未来真的出现：

```text
同一个系统同时保存 admin/user/read_only 三组账号
```

再允许一个 record 下多 credential。

当前数据库模型已经支持多条 `rd_credentials`，不需要重构表。

---

# 39. Web 保存事务

保存 HOST 时应该使用一个事务：

```text
BEGIN
  upsert rd_records
  upsert rd_credentials
  upsert rd_ssh_connections
COMMIT
```

保存 DATABASE：

```text
BEGIN
  upsert rd_records
  upsert rd_credentials
  upsert rd_db_connections
COMMIT
```

不要：

```text
POST /record
POST /credential
POST /ssh
```

让浏览器自己拼事务。

事务必须在后端完成。

---

# 40. alias 规则

alias 是 CLI 的关键字段。

必须：

```text
全局唯一
忽略大小写唯一
```

建议格式：

```text
^[a-zA-Z0-9][a-zA-Z0-9._-]*$
```

例如：

```text
n33edm
mysql-test
prod.db
bastion_a
```

不要允许空格。

CLI：

```bash
relay connect n33edm
```

才能稳定。

---

# 41. 本地开发流程

第一次：

```bash
mise install
mise run setup
```

终端 1：

```bash
mise run dev-backend
```

终端 2：

```bash
mise run dev-frontend
```

打开：

```text
http://127.0.0.1:5173
```

API：

```text
http://127.0.0.1:17321/api/health
```

---

# 42. API 联调

检查 health：

```bash
curl http://127.0.0.1:17321/api/health
```

结果：

```json
{"ok":true}
```

查询记录：

```bash
curl http://127.0.0.1:17321/api/records
```

---

# 43. 编译前端

执行：

```bash
mise run build-frontend
```

此时生成：

```text
internal/web/dist/
├── index.html
└── assets/
```

---

# 44. 构建 Go Binary

执行：

```bash
mise run build
```

输出：

```text
build/relay
```

检查：

```bash
./build/relay --help
```

例如：

```text
Local account and connection manager

Usage:
  relay [command]

Available Commands:
  connect
  list
  search
  show
  web
```

---

# 45. 验证前端已嵌入 Binary

构建完成后：

```bash
./build/relay web
```

访问：

```text
http://127.0.0.1:17321
```

此时不需要：

```text
Node
pnpm
Vite
frontend 目录
```

用户只有 `relay` 即可。

---

# 46. Cross Compile

因为 SQLite 选择 `modernc.org/sqlite`，可以避免 CGO 依赖，跨平台构建更简单。

macOS Apple Silicon：

```bash
GOOS=darwin GOARCH=arm64 \
  go build -trimpath -ldflags='-s -w' \
  -o build/relay-darwin-arm64 ./cmd/relay
```

macOS Intel：

```bash
GOOS=darwin GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' \
  -o build/relay-darwin-amd64 ./cmd/relay
```

Linux AMD64：

```bash
GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' \
  -o build/relay-linux-amd64 ./cmd/relay
```

Linux ARM64：

```bash
GOOS=linux GOARCH=arm64 \
  go build -trimpath -ldflags='-s -w' \
  -o build/relay-linux-arm64 ./cmd/relay
```

Windows：

```bash
GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags='-s -w' \
  -o build/relay-windows-amd64.exe ./cmd/relay
```

---

# 47. .gitignore

创建：

```gitignore
.DS_Store
.idea/
.vscode/

build/

frontend/node_modules/
frontend/dist/

# Generated Vue assets are rebuilt before release.
internal/web/dist/*
!internal/web/dist/placeholder.txt

# Local relay data
.relay/
*.db
*.db-shm
*.db-wal
```

如果希望把构建后的前端文件提交到 Git，则删除 `internal/web/dist/*` 规则。

更推荐：不提交构建产物。

---

# 48. 测试建议

第一阶段测试重点不是 HTTP Controller，而是业务解析。

必须测试：

### Record

```text
alias 唯一
alias 大小写冲突
非法 category
```

### Credential

```text
PASSWORD -> secret_value
SSH_KEY -> key_path
SSH_KEY 未填 key_path -> error
```

### SSH Route

```text
route 按 seq 排序
hop 必须是 HOST
重复 seq 拒绝
目标 host 不存在
hop host 不存在
hop host 配了另一个 route -> 第一版拒绝
```

### Database

```text
MYSQL 默认 3306
POSTGRESQL 默认 5432
不支持的 db type 拒绝
```

### CLI

```text
alias 不存在
ACCOUNT 执行 connect -> 拒绝
HOST connect -> 输出 SSH plan / 未启用提示
DATABASE connect -> 调用对应客户端
```

---

# 49. 推荐的第一阶段开发顺序

不要前后端一起乱写。

严格按下面顺序推进。

## Phase 0：工程骨架

完成：

```text
mise.toml
go.mod
目录结构
storage.Open()
relay --help
relay web
/api/health
Vue3 空页面
```

验收：

```bash
mise run build
./build/relay web
```

能打开页面。

---

## Phase 1：Record + Credential

完成：

```text
rd_records
rd_credentials
RecordRepository
RecordService
list/search/show
ACCOUNT CRUD
Web ACCOUNT 页面
```

验收：

```bash
relay list
relay search gitlab
relay show gitlab-test
```

ACCOUNT 页面能保存账号密码。

---

## Phase 2：HOST

完成：

```text
rd_ssh_connections
HOST CRUD
PASSWORD
SSH_KEY
SSH Plan
```

验收：

```bash
relay show n33edm
relay connect n33edm
```

第一阶段 `connect` 不真正启动 SSH，但必须把目标配置完整解析出来。

---

## Phase 3：Jump Route

完成：

```text
rd_ssh_routes
rd_ssh_route_hops
RouteRepository
RouteService
Route Web 页面
多跳解析
```

测试：

```text
bastion-a
  ↓
bastion-b
  ↓
n33edm
```

输出的 `SSHPlan.Hops` 顺序正确。

---

## Phase 4：DATABASE

完成：

```text
rd_db_connections
MYSQL
POSTGRESQL
DB Web 页面
relay connect <db alias>
```

验收：

```bash
relay connect index-mysql-test
relay connect index-pg-test
```

能够进入数据库终端客户端。

---

## Phase 5：SSH 真正启动

等前面的数据模型和 Route 解析都验证稳定后再做。

实现：

```text
SystemSSHLauncher
```

第一步支持：

```text
直连
单个 SSH Key
普通 ProxyJump
```

第二步再支持：

```text
每个 hop 不同 Key
SSH config 生成
SSH_ASKPASS
密码自动认证
```

不要第一版一次解决完。

---

# 50. SSH 真正启动时的建议方案

虽然第一阶段先不做，但架构应提前确定。

推荐：

```text
relay
   │
   │ resolve
   ▼
SSHPlan
   │
   │ render
   ▼
Temporary OpenSSH Config
   │
   ▼
system ssh
```

为什么不用一直拼：

```bash
ssh -J ... -i ...
```

因为多跳以后很快出现：

```text
Hop A 使用 key-a
Hop B 使用 key-b
Target 使用 key-c
不同 username
不同 port
ProxyJump
known_hosts
IdentityAgent
```

临时 config 更合适：

```text
Host relay-hop-1
    HostName 10.0.0.10
    Port 22
    User otis
    IdentityFile ~/.ssh/key-a

Host relay-hop-2
    HostName 10.0.1.10
    Port 22
    User root
    IdentityFile ~/.ssh/key-b
    ProxyJump relay-hop-1

Host relay-target
    HostName 172.17.113.33
    Port 22
    User app
    IdentityFile ~/.ssh/key-c
    ProxyJump relay-hop-2
```

调用：

```bash
ssh -F /tmp/relay-xxxx.conf relay-target
```

这样 relay 仍然不实现 SSH 协议，而是利用成熟 OpenSSH。

---

# 51. 不建议现在做的内容

第一版明确不做：

```text
登录系统
多用户
云同步
服务端
浏览器插件
SSH 协议实现
数据库驱动内置
远程凭证同步
复杂权限体系
全文搜索
复杂标签系统
审计中心
自动发现服务器
自动扫描 ~/.ssh/config
密码加密体系
插件系统
```

这些都可以以后根据真实需求增加。

---

# 52. 第一版最终产品形态

开发环境：

```text
mise
├── go
├── node
└── pnpm
```

代码：

```text
Go
├── CLI
├── HTTP
├── SQLite
├── SSH Plan
├── DB Launcher
└── Vue embed
```

运行时：

```text
relay
  │
  ├── ~/.relay/relay.db
  │
  ├── relay list
  ├── relay search
  ├── relay show
  ├── relay connect
  └── relay web
```

用户不需要安装 Java Runtime。

Web 模式不需要 Node。

Node/pnpm 只存在于开发阶段。

---

# 53. 第一阶段完成标准

当以下场景全部成立，就可以认为第一版完成。

### ACCOUNT

```text
Web 创建 GitLab 账号
→ 保存 username/password
→ relay show gitlab-test
→ 正常展示，密码默认隐藏
```

### HOST + SSH Key

```text
Web 创建 n33edm
→ host=172.17.113.33
→ username=otis
→ key=~/.ssh/id_ed25519
→ relay show n33edm
→ 正确显示
```

### Jump Host

```text
创建 bastion-a
创建 bastion-b
创建 prod-route
  hop1=bastion-a
  hop2=bastion-b

n33edm.route=prod-route

relay connect n33edm
```

输出：

```text
bastion-a
   ↓
bastion-b
   ↓
n33edm
```

并能看到每一跳对应：

```text
host
port
username
authType
keyPath
```

### DATABASE

```text
Web 创建 index-mysql-test
→ 保存 host/port/database/user/password
→ relay connect index-mysql-test
→ 启动 mysql client
```

### 单文件构建

```bash
mise run build
```

得到：

```text
build/relay
```

复制 `relay` 到另一台同平台机器后，只要系统存在所需的数据库/SSH 客户端，就可以使用 CLI 和 Web UI。

---

# 54. 最终建议

relay 第一版不要追求“完整密码管理器”。

先把真正有价值的链路做通：

```text
保存
  ↓
搜索
  ↓
alias
  ↓
解析连接配置
  ↓
打开目标终端
```

数据模型保持：

```text
Record
  ├── Credential
  ├── SSH Connection
  │      └── Route
  │             └── Hops -> HOST
  └── DB Connection
```

这套结构已经足够支撑：

```text
账号密码
Host
SSH Key
跳板机
多跳
MySQL
PostgreSQL
CLI
Web UI
```

同时又没有把项目做成一个复杂的平台。

后续最自然的增强顺序是：

```text
SSH Launcher
    ↓
临时 OpenSSH Config
    ↓
DB SSH Tunnel
    ↓
系统 Keychain
    ↓
Import / Export
```

在这些能力真正出现之前，不建议继续增加架构层次。
