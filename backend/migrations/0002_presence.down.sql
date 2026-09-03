DROP TRIGGER IF EXISTS trg_presence_audit_log_no_delete ON presence_audit_log;
DROP TRIGGER IF EXISTS trg_presence_audit_log_no_update ON presence_audit_log;
DROP FUNCTION IF EXISTS presence_audit_log_immutable();
DROP TABLE IF EXISTS presence_audit_log;
DROP TABLE IF EXISTS user_blocks;
DROP TABLE IF EXISTS user_privacy_settings;
