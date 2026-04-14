DROP INDEX IF EXISTS apollo.idx_schedule_block_exceptions_block_id_date;

DROP TABLE IF EXISTS apollo.schedule_block_exceptions;

DROP INDEX IF EXISTS apollo.idx_schedule_blocks_resource_key;

DROP INDEX IF EXISTS apollo.idx_schedule_blocks_facility_scope_status;

DROP TABLE IF EXISTS apollo.schedule_blocks;

DROP INDEX IF EXISTS apollo.idx_schedule_resource_edges_related_resource_key;

DROP TABLE IF EXISTS apollo.schedule_resource_edges;

DROP INDEX IF EXISTS apollo.idx_schedule_resources_facility_key;

DROP TABLE IF EXISTS apollo.schedule_resources;
