ALTER TABLE msg_internal DROP CONSTRAINT IF EXISTS fk_msg_internal_msg_record;
ALTER TABLE msg_internal_reads DROP CONSTRAINT IF EXISTS fk_msg_internal_reads_msg_internal;
