PRAGMA foreign_keys = ON;

-- =========================================================
-- 1. 记录主表
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_records (
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

CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_records_alias ON rd_records (alias COLLATE NOCASE)
WHERE deleted = 'N' AND category <> 'NOTE';

CREATE INDEX IF NOT EXISTS idx_rd_records_category ON rd_records (category);
CREATE INDEX IF NOT EXISTS idx_rd_records_deleted ON rd_records (deleted);
CREATE INDEX IF NOT EXISTS idx_rd_records_category_deleted ON rd_records (category, deleted);

-- =========================================================
-- 2. 凭据
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_credentials (
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
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_ssh_routes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    deleted TEXT NOT NULL DEFAULT 'N' CHECK (deleted IN ('Y', 'N')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_rd_ssh_routes_name ON rd_ssh_routes (name COLLATE NOCASE)
WHERE deleted = 'N';

CREATE INDEX IF NOT EXISTS idx_rd_ssh_routes_deleted ON rd_ssh_routes (deleted);

-- =========================================================
-- 4. SSH Connection
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_ssh_connections (
    record_id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 22 CHECK (port > 0 AND port <= 65535),
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
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_ssh_route_hops (
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
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_db_connections (
    record_id TEXT PRIMARY KEY,
    db_type TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port > 0 AND port <= 65535),
    database_name TEXT,
    credential_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE,
    FOREIGN KEY (credential_id) REFERENCES rd_credentials (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_rd_db_connections_credential ON rd_db_connections (credential_id);
CREATE INDEX IF NOT EXISTS idx_rd_db_connections_type ON rd_db_connections (db_type);

-- =========================================================
-- 7. Tags
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE,
    created_at TEXT NOT NULL,
    UNIQUE (name)
);

CREATE TABLE IF NOT EXISTS rd_record_tags (
    record_id TEXT NOT NULL,
    tag_id TEXT NOT NULL,
    PRIMARY KEY (record_id, tag_id),
    FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES rd_tags (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rd_record_tags_tag ON rd_record_tags (tag_id);

-- =========================================================
-- 8. Connection Usage
--
-- 仅记录 HOST / DATABASE 的使用情况，用于 CLI picker 的最近使用排序。
-- =========================================================
CREATE TABLE IF NOT EXISTS rd_record_usage (
    record_id TEXT PRIMARY KEY,
    last_connected_at TEXT NOT NULL,
    connect_count INTEGER NOT NULL DEFAULT 0 CHECK (connect_count >= 0),
    FOREIGN KEY (record_id) REFERENCES rd_records (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_rd_record_usage_last_connected
ON rd_record_usage (last_connected_at DESC);
