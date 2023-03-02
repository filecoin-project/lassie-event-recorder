create table if not exists retrieval_events(
  retrieval_id uuid not null,
  instance_id character varying(64) not null,
  cid character varying(256) not null,
  storage_provider_id character varying(256) ,
  phase character varying(15) not null,
  phase_start_time timestamp with time zone not null,
  event_name character varying(32) not null,
  event_time timestamp with time zone not null,
  event_details jsonb
);