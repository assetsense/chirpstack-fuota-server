create table deployment (
    id uuid primary key not null,
    created_at timestamp with time zone not null,
    updated_at timestamp with time zone not null,
    mc_group_setup_completed_at timestamp with time zone null,
    mc_session_completed_at timestamp with time zone null,
    frag_session_setup_completed_at timestamp with time zone null,
    enqueue_completed_at timestamp with time zone null,
    frag_status_completed_at timestamp with time zone null
);

create table deployment_device (
    deployment_id uuid not null references deployment on delete cascade,
    dev_eui bytea not null,
    created_at timestamp with time zone not null,
    updated_at timestamp with time zone not null,

    mc_group_setup_completed_at timestamp with time zone null,
    mc_session_completed_at timestamp with time zone null,
    frag_session_setup_completed_at timestamp with time zone null,
    frag_status_completed_at timestamp with time zone null,

    primary key (deployment_id, dev_eui)
);

create table deployment_log (
    id bigserial primary key,
    created_at timestamp with time zone not null,
    deployment_id uuid not null references deployment on delete cascade,
    dev_eui bytea not null,
    f_port smallint not null,
    command varchar(50) not null,
    fields hstore 
);

CREATE TABLE device_profile (
    profileid BIGINT PRIMARY KEY,
    region VARCHAR(255),
    macversion VARCHAR(255),
    regionparameter VARCHAR(255)
);

CREATE TABLE device (
    deviceid BIGINT PRIMARY KEY,
    devicecode VARCHAR(255),
    modelid BIGINT,
    profileid BIGINT,
    firmwareversion VARCHAR(255),
    status BIGINT,
    CONSTRAINT fk_device_profile FOREIGN KEY (profileid) REFERENCES device_profile(profileid)
);


create index idx_deployment_log_deployment_id on deployment_log(deployment_id);
create index idx_deployment_log_dev_eui on deployment_log(dev_eui);
