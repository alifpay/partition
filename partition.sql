-- audit log for user actions
CREATE TABLE user_audit_log (
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    action SMALLINT NOT NULL,
    user_id TEXT NOT NULL,
    details JSONB
) WITH (
    tsdb.hypertable,
    tsdb.partition_column = 'created_at',
    tsdb.segmentby       = 'user_id',
    tsdb.orderby         = 'created_at DESC'
);

CREATE INDEX idx_user_action
    ON user_audit_log(action, user_id, created_at);

insert into user_audit_log(action, user_id, details)
values 
(1, 'user_123', '{"login": "successful"}'),
(2, 'user_456', '{"file_upload": "document.pdf"}'),
(1, 'user_123', '{"logout": "successful"}'),
(3, 'user_789', '{"password_change": "successful"}');

