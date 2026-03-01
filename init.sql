-- English comments for the database schema
-- This table tracks tasks sent to OpenClaw and their resolution status

CREATE TABLE IF NOT EXISTS claw_tasks (
    id SERIAL PRIMARY KEY,
    task_id VARCHAR(50) UNIQUE NOT NULL,      -- Business logic ID
    sender_id VARCHAR(100) NOT NULL,          -- Lark User OpenID
    chat_id VARCHAR(100) NOT NULL,            -- Chat/Group ID for reply
    raw_content TEXT,                         -- Original message from Lark
    task_status VARCHAR(20) DEFAULT 'pending',-- pending, processing, completed, failed
    ai_response TEXT,                         -- Final answer from OpenClaw
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster lookups during polling
CREATE INDEX idx_task_id ON claw_tasks(task_id);
