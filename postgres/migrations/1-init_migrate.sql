
-- +migrate Up
CREATE TABLE IF NOT EXISTS mempool (
    time timestamp,
    first_seen_time timestamp,
    number_of_transactions INT,
    voters INT,
    tickets INT,
    revocations INT,
    size INT,
    total_fee FLOAT8,
    total FLOAT8,
    PRIMARY KEY (time)
);
CREATE TABLE IF NOT EXISTS block (
    height INT,
    receive_time timestamp,
    internal_timestamp timestamp,
    hash VARCHAR(512),
    PRIMARY KEY (height)
);
CREATE TABLE IF NOT EXISTS vote (
    hash VARCHAR(128),
    voting_on INT8,
    block_hash VARCHAR(128),
    receive_time timestamp,
    block_receive_time timestamp,
    targeted_block_time timestamp,
    validator_id INT,
    validity VARCHAR(128),
    PRIMARY KEY (hash)
);
CREATE TABLE IF NOT EXISTS vsp (
    id SERIAL PRIMARY KEY,
    name TEXT,
    api_enabled BOOLEAN,
    api_versions_supported INT8[],
    network TEXT,
    url TEXT,
    launched TIMESTAMPTZ
);
CREATE TABLE IF NOT EXISTS vsp_tick (
    id SERIAL PRIMARY KEY,
    vsp_id INT REFERENCES vsp(id) NOT NULL,
    immature INT NOT NULL,
    live INT NOT NULL,
    voted INT NOT NULL,
    missed INT NOT NULL,
    pool_fees FLOAT NOT NULL,
    proportion_live FLOAT NOT NULL,
    proportion_missed FLOAT NOT NULL,
    user_count INT NOT NULL,
    users_active INT NOT NULL,
    time TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS vsp_tick_idx ON vsp_tick (vsp_id,immature,live,voted,missed,pool_fees,proportion_live,proportion_missed,user_count,users_active, time);
CREATE TABLE IF NOT EXISTS exchange (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS exchange_tick (
    id SERIAL PRIMARY KEY,
    exchange_id INT REFERENCES exchange(id) NOT NULL,
    interval INT NOT NULL,
    high FLOAT NOT NULL,
    low FLOAT NOT NULL,
    open FLOAT NOT NULL,
    close FLOAT NOT NULL,
    volume FLOAT NOT NULL,
    currency_pair TEXT NOT NULL,
    time TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS exchange_tick_idx ON exchange_tick (exchange_id, interval, currency_pair, time);
CREATE TABLE IF NOT EXISTS pow_data (
    time INT,
    pool_hashrate VARCHAR(25),
    workers INT,
    coin_price VARCHAR(25),
    btc_price VARCHAR(25),
    source VARCHAR(25),
    PRIMARY KEY (time, source)
);
CREATE TABLE IF NOT EXISTS reddit (
    date timestamp,
    subreddit VARCHAR(256) NOT NULL,
    subscribers INT NOT NULL,
    active_accounts INT NOT NULL,
    PRIMARY KEY (date)
);
CREATE TABLE IF NOT EXISTS twitter (
    date timestamp,
    handle VARCHAR(256) NOT NULL,
    followers INT NOT NULL,
    PRIMARY KEY (date)
);
CREATE TABLE IF NOT EXISTS youtube (
    date timestamp,
    subscribers INT NOT NULL,
    view_count INT NOT NULL,
    channel VARCHAR(256) NOT NULL,
    PRIMARY KEY (date)
);
CREATE TABLE IF NOT EXISTS github (
    date timestamp,
    repository VARCHAR(256) NOT NULL,
    stars INT NOT NULL,
    folks INT NOT NULL,
    PRIMARY KEY (date)
);
CREATE TABLE If NOT EXISTS network_snapshot (
    timestamp INT8 NOT NULL,
    height INT8 NOT NULL,
    node_count INT NOT NULL,
    reachable_nodes INT NOT NULL,
    oldest_node VARCHAR(256) NOT NULL DEFAULT '',
    oldest_node_timestamp INT8 NOT NULL DEFAULT 0,
    latency INT NOT NULL DEFAULT 0,
    PRIMARY KEY (timestamp)
);
CREATE TABLE If NOT EXISTS node (
    address VARCHAR(256) NOT NULL PRIMARY KEY,
    ip_version INT NOT NULL,
    country VARCHAR(256) NOT NULL,
    region VARCHAR(256) NOT NULL,
    city VARCHAR(256) NOT NULL,
    zip VARCHAR(256) NOT NULL,
    last_attempt INT8 NOT NULL,
    last_seen INT8 NOT NULL,
    last_success INT8 NOT NULL,
    failure_count INT NOT NULL DEFAULT 0,
    is_dead BOOLEAN NOT NULL,
    connection_time INT8 NOT NULL,
    protocol_version INT NOT NULL,
    user_agent VARCHAR(256) NOT NULL,
    services VARCHAR(256) NOT NULL,
    starting_height INT8 NOT NULL,
    current_height INT8 NOT NULL
);
CREATE TABLE If NOT EXISTS heartbeat (
    timestamp INT8 NOT NULL,
    node_id VARCHAR(256) NOT NULL REFERENCES node(address),
    last_seen INT8 NOT NULL,
    latency INT NOT NULL,
    current_height INT8 NOT NULL,
    PRIMARY KEY (timestamp, node_id)
);
-- +migrate Down
DROP INDEX IF EXISTS vsp_tick_idx;
DROP INDEX IF EXISTS exchange_tick_idx;
DROP TABLE IF EXISTS vsp_tick, vsp, exchange_tick, exchange, pow_data, mempool, block, vote, reddit, github, twitter, youtube, comm_stat, network_snapshot, heartbeat, node;
