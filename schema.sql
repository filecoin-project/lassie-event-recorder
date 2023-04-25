create table if not exists retrieval_events(
  retrieval_id uuid not null,
  instance_id character varying(64) not null,
  cid character varying(256) not null,
  storage_provider_id character varying(256),
  phase character varying(15) not null,
  phase_start_time timestamp with time zone not null,
  event_name character varying(32) not null,
  event_time timestamp with time zone not null,
  event_details jsonb
);

create table if not exists aggregate_retrieval_events(
  retrieval_id uuid not null UNIQUE,
  instance_id character varying(64) not null,
  storage_provider_id character varying(256),
  time_to_first_byte int8,
  bandwidth_bytes_sec int8,
  bytes_transferred int8,
  success boolean not null,
  start_time timestamp with time zone not null,
  end_time timestamp with time zone not null,
  time_to_first_indexer_result int8,                      
	indexer_candidates_received integer,
	indexer_candidates_filtered integer,
	protocols_allowed        varchar[256][],    
	protocols_attempted      varchar[256][]
);

create table if not exists retrieval_attempts(
  retrieval_id uuid not null,
  storage_provider_id character varying(256),
  time_to_first_byte int8,
  error character varying(256),
  FOREIGN KEY (retrieval_id) REFERENCES aggregate_retrieval_events (retrieval_id)
);
