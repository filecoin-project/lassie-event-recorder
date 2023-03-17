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
  retrieval_id uuid not null,
  instance_id character varying(64) not null,
  storage_provider_id character varying(256),
  time_to_first_byte_ms integer,
  bandwidth_bytes_sec integer,
  success boolean not null,
  start_time timestamp with time zone not null,
  end_time timestamp with time zone not null
);