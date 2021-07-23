CREATE TABLE domain (
    r_data VARCHAR(255) PRIMARY KEY,
    r_name VARCHAR(255) NOT NULL,
    r_class TINYINT DEFAULT 1,
    r_type TINYINT NOT NULL,
    r_ttl INTEGER NOT NULL,
    expired_at BIGINT DEFAULT 0
);
