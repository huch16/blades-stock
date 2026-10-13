ALTER TABLE agent_memories ADD COLUMN embedding BLOB;
CREATE INDEX IF NOT EXISTS idx_agent_memories_session_type ON agent_memories(session_id, type);
CREATE INDEX IF NOT EXISTS idx_agent_memories_created_at ON agent_memories(created_at);