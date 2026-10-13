-- v20261013_create_index_manifests.sql
-- 创建 index_manifests 表并建立唯一约束（doc_id, backend）
CREATE TABLE IF NOT EXISTS index_manifests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  doc_id TEXT NOT NULL,
  source TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  backend TEXT NOT NULL,
  indexed_at DATETIME NOT NULL,
  created_at DATETIME DEFAULT (datetime('now'))
);

-- 唯一索引，避免同一 doc 在同一 backend 重复插入
CREATE UNIQUE INDEX IF NOT EXISTS ux_index_manifests_doc_backend
  ON index_manifests(doc_id, backend);

-- 辅助索引按 doc_id 快速检索
CREATE INDEX IF NOT EXISTS idx_index_manifests_docid
  ON index_manifests(doc_id);
