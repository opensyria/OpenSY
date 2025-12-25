-- OpenSY Pool Database Schema
-- PostgreSQL 15+

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- MINERS
-- ============================================================================
CREATE TABLE miners (
    id              SERIAL PRIMARY KEY,
    address         VARCHAR(64) NOT NULL UNIQUE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen       TIMESTAMP WITH TIME ZONE,
    total_hashes    BIGINT DEFAULT 0,
    total_shares    BIGINT DEFAULT 0,
    invalid_shares  BIGINT DEFAULT 0,
    balance         DECIMAL(20, 8) DEFAULT 0,
    paid_total      DECIMAL(20, 8) DEFAULT 0,
    
    -- Metadata
    email           VARCHAR(255),           -- Optional notifications
    min_payout      DECIMAL(20, 8),         -- Custom min payout (NULL = pool default)
    
    CONSTRAINT valid_address CHECK (address ~ '^syl1[a-z0-9]{38,}$' OR address ~ '^F[a-zA-Z0-9]{33}$')
);

CREATE INDEX idx_miners_address ON miners(address);
CREATE INDEX idx_miners_last_seen ON miners(last_seen);

-- ============================================================================
-- WORKERS
-- ============================================================================
CREATE TABLE workers (
    id              SERIAL PRIMARY KEY,
    miner_id        INTEGER NOT NULL REFERENCES miners(id) ON DELETE CASCADE,
    name            VARCHAR(64) NOT NULL,
    first_seen      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen       TIMESTAMP WITH TIME ZONE,
    hashrate_avg    BIGINT DEFAULT 0,       -- H/s (1 minute average)
    hashrate_5m     BIGINT DEFAULT 0,       -- H/s (5 minute average)
    difficulty      BIGINT DEFAULT 10000,   -- Current vardiff
    shares_valid    BIGINT DEFAULT 0,
    shares_invalid  BIGINT DEFAULT 0,
    
    UNIQUE(miner_id, name)
);

CREATE INDEX idx_workers_miner ON workers(miner_id);
CREATE INDEX idx_workers_last_seen ON workers(last_seen);

-- ============================================================================
-- SHARES (Partitioned by time for performance)
-- ============================================================================
CREATE TABLE shares (
    id              BIGSERIAL,
    worker_id       INTEGER NOT NULL REFERENCES workers(id) ON DELETE CASCADE,
    height          INTEGER NOT NULL,
    difficulty      BIGINT NOT NULL,        -- Network difficulty
    share_diff      BIGINT NOT NULL,        -- Share difficulty
    timestamp       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    is_valid        BOOLEAN DEFAULT TRUE,
    is_block        BOOLEAN DEFAULT FALSE,
    block_hash      VARCHAR(64),
    nonce           VARCHAR(16),            -- Submitted nonce (hex)
    ip_address      INET,
    
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create partitions for the next 6 months
CREATE TABLE shares_2025_12 PARTITION OF shares
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
CREATE TABLE shares_2026_01 PARTITION OF shares
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE shares_2026_02 PARTITION OF shares
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE shares_2026_03 PARTITION OF shares
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE shares_2026_04 PARTITION OF shares
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE shares_2026_05 PARTITION OF shares
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_shares_worker ON shares(worker_id);
CREATE INDEX idx_shares_height ON shares(height);
CREATE INDEX idx_shares_block ON shares(is_block) WHERE is_block = TRUE;

-- ============================================================================
-- BLOCKS
-- ============================================================================
CREATE TABLE blocks (
    id              SERIAL PRIMARY KEY,
    height          INTEGER NOT NULL UNIQUE,
    hash            VARCHAR(64) NOT NULL UNIQUE,
    prev_hash       VARCHAR(64) NOT NULL,
    reward          DECIMAL(20, 8) NOT NULL,
    finder_id       INTEGER REFERENCES workers(id),
    timestamp       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    confirmations   INTEGER DEFAULT 0,
    orphaned        BOOLEAN DEFAULT FALSE,
    mature          BOOLEAN DEFAULT FALSE,  -- 100+ confirmations
    
    -- Extra metadata
    difficulty      DECIMAL(30, 10),
    tx_count        INTEGER,
    size_bytes      INTEGER
);

CREATE INDEX idx_blocks_height ON blocks(height);
CREATE INDEX idx_blocks_mature ON blocks(mature) WHERE mature = FALSE;

-- ============================================================================
-- PAYOUTS
-- ============================================================================
CREATE TABLE payouts (
    id              SERIAL PRIMARY KEY,
    miner_id        INTEGER NOT NULL REFERENCES miners(id),
    amount          DECIMAL(20, 8) NOT NULL,
    fee             DECIMAL(20, 8) DEFAULT 0,  -- Transaction fee
    txid            VARCHAR(64),
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    sent_at         TIMESTAMP WITH TIME ZONE,
    confirmed_at    TIMESTAMP WITH TIME ZONE,
    status          VARCHAR(20) DEFAULT 'pending',
    
    -- Status: pending, sent, confirmed, failed
    error_message   TEXT,
    
    CONSTRAINT positive_amount CHECK (amount > 0)
);

CREATE INDEX idx_payouts_miner ON payouts(miner_id);
CREATE INDEX idx_payouts_status ON payouts(status);
CREATE INDEX idx_payouts_txid ON payouts(txid) WHERE txid IS NOT NULL;

-- ============================================================================
-- PAYOUT DETAILS (PPLNS breakdown per block)
-- ============================================================================
CREATE TABLE payout_details (
    id              SERIAL PRIMARY KEY,
    payout_id       INTEGER REFERENCES payouts(id) ON DELETE CASCADE,
    block_id        INTEGER REFERENCES blocks(id),
    miner_id        INTEGER NOT NULL REFERENCES miners(id),
    shares          BIGINT NOT NULL,
    difficulty      BIGINT NOT NULL,
    percentage      DECIMAL(10, 6) NOT NULL,
    amount          DECIMAL(20, 8) NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_payout_details_payout ON payout_details(payout_id);
CREATE INDEX idx_payout_details_miner ON payout_details(miner_id);
CREATE INDEX idx_payout_details_block ON payout_details(block_id);

-- ============================================================================
-- POOL STATS (Time-series for dashboard)
-- ============================================================================
CREATE TABLE pool_stats (
    id              SERIAL PRIMARY KEY,
    timestamp       TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    hashrate        BIGINT,                 -- Total pool H/s
    workers_online  INTEGER,
    miners_online   INTEGER,
    difficulty      DECIMAL(30, 10),        -- Network difficulty
    height          INTEGER,
    blocks_found    INTEGER,                -- Total blocks ever
    shares_total    BIGINT                  -- Total shares ever
);

CREATE INDEX idx_pool_stats_time ON pool_stats(timestamp);

-- ============================================================================
-- POOL CONFIGURATION
-- ============================================================================
CREATE TABLE pool_config (
    key             VARCHAR(64) PRIMARY KEY,
    value           TEXT NOT NULL,
    description     TEXT,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Default configuration
INSERT INTO pool_config (key, value, description) VALUES
    ('pool_fee', '1.0', 'Pool fee percentage'),
    ('payout_scheme', 'PPLNS', 'Payout scheme: PPLNS, PPS, PROP'),
    ('pplns_window', '10000', 'PPLNS window size in shares'),
    ('min_payout', '100', 'Minimum payout in SYL'),
    ('payout_interval', '3600', 'Payout check interval in seconds'),
    ('vardiff_target_time', '30', 'Target seconds between shares'),
    ('vardiff_retarget', '120', 'Vardiff recalculation interval'),
    ('vardiff_min', '1000', 'Minimum share difficulty'),
    ('vardiff_max', '1000000000', 'Maximum share difficulty'),
    ('block_maturity', '100', 'Blocks required for maturity'),
    ('max_connections_per_ip', '100', 'Max Stratum connections per IP'),
    ('ban_threshold', '10', 'Invalid shares before ban'),
    ('ban_duration', '3600', 'Ban duration in seconds');

-- ============================================================================
-- BANS
-- ============================================================================
CREATE TABLE bans (
    id              SERIAL PRIMARY KEY,
    ip_address      INET NOT NULL,
    reason          TEXT,
    expires_at      TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_bans_ip ON bans(ip_address);
CREATE INDEX idx_bans_expires ON bans(expires_at);

-- ============================================================================
-- HELPER FUNCTIONS
-- ============================================================================

-- Function to update miner stats
CREATE OR REPLACE FUNCTION update_miner_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_valid THEN
        UPDATE workers SET 
            shares_valid = shares_valid + 1,
            last_seen = NOW()
        WHERE id = NEW.worker_id;
    ELSE
        UPDATE workers SET 
            shares_invalid = shares_invalid + 1,
            last_seen = NOW()
        WHERE id = NEW.worker_id;
    END IF;
    
    -- Update miner last_seen
    UPDATE miners SET last_seen = NOW()
    WHERE id = (SELECT miner_id FROM workers WHERE id = NEW.worker_id);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_miner_stats
    AFTER INSERT ON shares
    FOR EACH ROW
    EXECUTE FUNCTION update_miner_stats();

-- Function to create next month's partition
CREATE OR REPLACE FUNCTION create_shares_partition(target_date DATE)
RETURNS VOID AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := date_trunc('month', target_date);
    end_date := start_date + INTERVAL '1 month';
    partition_name := 'shares_' || to_char(start_date, 'YYYY_MM');
    
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF shares FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_date, end_date
    );
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- VIEWS
-- ============================================================================

-- Active miners (seen in last hour)
CREATE VIEW active_miners AS
SELECT 
    m.id,
    m.address,
    m.balance,
    COUNT(w.id) as worker_count,
    SUM(w.hashrate_avg) as total_hashrate,
    MAX(w.last_seen) as last_seen
FROM miners m
JOIN workers w ON w.miner_id = m.id
WHERE w.last_seen > NOW() - INTERVAL '1 hour'
GROUP BY m.id, m.address, m.balance;

-- Recent blocks
CREATE VIEW recent_blocks AS
SELECT 
    b.*,
    m.address as finder_address,
    w.name as finder_worker
FROM blocks b
LEFT JOIN workers w ON w.id = b.finder_id
LEFT JOIN miners m ON m.id = w.miner_id
ORDER BY b.height DESC
LIMIT 100;

-- ============================================================================
-- GRANTS (for pool application user)
-- ============================================================================
-- GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO pool_app;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO pool_app;

COMMENT ON DATABASE opensy_pool IS 'OpenSY Mining Pool Database';
