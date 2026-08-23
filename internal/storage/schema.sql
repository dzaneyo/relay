PRAGMA foreign_keys = ON;

-- =========================================================
-- 1. 记录主表
--
-- category:
--   NOTE      纯文本笔记（仅 Web 管理）
--   HOST      主机
--   DATABASE  数据库
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_records (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        alias TEXT NOT NULL,
        category TEXT NOT NULL CHECK (category IN ('NOTE', 'HOST', 'DATABASE')),
        notes TEXT NOT NULL DEFAULT '',
        favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1)),
        deleted TEXT NOT NULL DEFAULT 'N' CHECK (deleted IN ('Y', 'N')),
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    );

-- alias 只要求“未删除且可连接的记录”唯一；NOTE 固定使用空 alias。
-- 删除后允许重新创建相同 alias。
CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_records_alias ON rd_records (alias COLLATE NOCASE)
WHERE
    deleted = 'N' AND category <> 'NOTE';

CREATE INDEX IF NOT EXISTS idx_rd_records_category ON rd_records (category);

CREATE INDEX IF NOT EXISTS idx_rd_records_deleted ON rd_records (deleted);

CREATE INDEX IF NOT EXISTS idx_rd_records_category_deleted ON rd_records (category, deleted);

-- =========================================================
-- 2. 凭据
--
-- 一个 Record 可以有多个 Credential，例如：
--
-- HOST:
--   default / PASSWORD
--   root    / SSH_KEY
--
-- DATABASE:
--   readonly / PASSWORD
--   admin    / PASSWORD
--
-- auth_type:
--   PASSWORD
--   SSH_KEY
--
-- secret_value:
--   PASSWORD 时保存密码
--
-- key_path:
--   SSH_KEY 时保存本机私钥路径
--   例如 ~/.ssh/id_ed25519
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_credentials (
        id TEXT PRIMARY KEY,
        record_id TEXT NOT NULL,
        label TEXT NOT NULL DEFAULT 'default',
        username TEXT,
        auth_type TEXT NOT NULL CHECK (auth_type IN ('NONE', 'PASSWORD', 'SSH_KEY')),
        secret_value TEXT,
        key_path TEXT,
        deleted TEXT NOT NULL DEFAULT 'N' CHECK (deleted IN ('Y', 'N')),
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE
    );

CREATE INDEX IF NOT EXISTS idx_rd_credentials_record ON rd_credentials (record_id);

CREATE INDEX IF NOT EXISTS idx_rd_credentials_record_deleted ON rd_credentials (record_id, deleted);

-- =========================================================
-- 3. SSH 路由
--
-- 一个 Route 表示一条跳板机链路，例如：
--
-- route:
--   prod-route
--
-- hops:
--   1 -> bastion-a
--   2 -> bastion-b
--
-- 最终：
--
-- local
--   -> bastion-a
--   -> bastion-b
--   -> target
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_ssh_routes (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL DEFAULT '',
        deleted TEXT NOT NULL DEFAULT 'N' CHECK (deleted IN ('Y', 'N')),
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    );

-- 已删除 Route 不占用名称
CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_ssh_routes_name ON rd_ssh_routes (name COLLATE NOCASE)
WHERE
    deleted = 'N';

CREATE INDEX IF NOT EXISTS idx_rd_ssh_routes_deleted ON rd_ssh_routes (deleted);

-- =========================================================
-- 4. SSH Connection
--
-- 仅 HOST 类型 Record 使用。
--
-- credential_id:
--   目标主机使用的凭据
--
-- route_id:
--   可选跳板路由
--
-- 例如：
--
-- rd_records
--   alias = n33edm
--   category = HOST
--
-- rd_ssh_connections
--   host = 172.17.33.10
--   port = 22
--   credential_id = xxx
--   route_id = route-prod
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_ssh_connections (
        record_id TEXT PRIMARY KEY,
        host TEXT NOT NULL,
        port INTEGER NOT NULL DEFAULT 22 CHECK (
            port > 0
            AND port <= 65535
        ),
        credential_id TEXT,
        route_id TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE,
        FOREIGN KEY (credential_id) REFERENCES rd_credentials (id) ON DELETE SET NULL,
        FOREIGN KEY (route_id) REFERENCES rd_ssh_routes (id) ON DELETE SET NULL
    );

CREATE INDEX IF NOT EXISTS idx_rd_ssh_connections_credential ON rd_ssh_connections (credential_id);

CREATE INDEX IF NOT EXISTS idx_rd_ssh_connections_route ON rd_ssh_connections (route_id);

-- =========================================================
-- 5. SSH Route Hop
--
-- 每个 hop 引用一个 HOST Record。
--
-- 例如：
--
-- route_id = route-prod
--
-- seq | host_record_id
-- ----+---------------
-- 1   | bastion-a
-- 2   | bastion-b
--
-- 不使用软删除。
-- 删除/修改路由时直接维护关系数据。
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_ssh_route_hops (
        route_id TEXT NOT NULL,
        seq INTEGER NOT NULL CHECK (seq > 0),
        host_record_id TEXT NOT NULL,
        PRIMARY KEY (route_id, seq),
        FOREIGN KEY (route_id) REFERENCES rd_ssh_routes (id) ON DELETE CASCADE,
        FOREIGN KEY (host_record_id) REFERENCES rd_records (id) ON DELETE RESTRICT
    );

CREATE INDEX IF NOT EXISTS idx_rd_ssh_route_hops_host ON rd_ssh_route_hops (host_record_id);

-- =========================================================
-- 6. Database Connection
--
-- 仅 DATABASE 类型 Record 使用。
--
-- 第一版建议支持：
--   MYSQL
--   POSTGRESQL
--
-- 后面可以继续增加：
--   ORACLE
--   CLICKHOUSE
--   DORIS
--   SQLSERVER
--   ...
--
-- credential_id:
--   数据库登录凭据
-- =========================================================
CREATE TABLE
    IF NOT EXISTS rd_db_connections (
        record_id TEXT PRIMARY KEY,
        db_type TEXT NOT NULL,
        host TEXT NOT NULL,
        port INTEGER NOT NULL CHECK (
            port > 0
            AND port <= 65535
        ),
        database_name TEXT,
        credential_id TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE,
        FOREIGN KEY (credential_id) REFERENCES rd_credentials (id) ON DELETE SET NULL
    );

CREATE INDEX IF NOT EXISTS idx_rd_db_connections_credential ON rd_db_connections (credential_id);

CREATE INDEX IF NOT EXISTS idx_rd_db_connections_type ON rd_db_connections (db_type);
