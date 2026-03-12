CREATE TABLE IF NOT EXISTS metrics (
    id         VARCHAR(255)     PRIMARY KEY,
    type       VARCHAR(10)      NOT NULL,
    delta      BIGINT,
    value      DOUBLE PRECISION,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
