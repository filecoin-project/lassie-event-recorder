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
  root_cid character varying(256),
  url_path text,
  instance_id character varying(64) not null,
  storage_provider_id character varying(256),
  time_to_first_byte bigint,
  bandwidth_bytes_sec bigint,
  bytes_transferred bigint,
  success boolean not null,
  start_time timestamp with time zone not null,
  end_time timestamp with time zone not null,
  time_to_first_indexer_result bigint,
  indexer_candidates_received integer,
  indexer_candidates_filtered integer,
  protocols_allowed        character varying[],
  protocols_attempted      character varying[],
  protocol_succeeded       character varying(256)
);

create table if not exists retrieval_attempts(
  retrieval_id uuid not null,
  storage_provider_id character varying(256),
  time_to_first_byte bigint,
  error text,
  protocol character varying(256),
  bytes_transferred bigint,
  FOREIGN KEY (retrieval_id) REFERENCES aggregate_retrieval_events (retrieval_id)
);
