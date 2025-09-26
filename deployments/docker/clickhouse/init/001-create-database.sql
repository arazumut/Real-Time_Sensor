-- TWINUP Sensor Data Database Initialization
-- Bu script ClickHouse container başlarken otomatik çalışır

-- Database oluştur
CREATE DATABASE IF NOT EXISTS sensor_data;

-- Sensor data tablosu - yüksek performanslı time-series data için optimize edilmiş
CREATE TABLE IF NOT EXISTS sensor_data.sensor_readings (
    device_id String,
    timestamp DateTime64(3, 'UTC'),
    temperature Float64,
    humidity Float64,
    pressure Float64,
    location_lat Float64,
    location_lng Float64,
    status LowCardinality(String),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

-- Device summary tablosu - aggregated data için
CREATE TABLE IF NOT EXISTS sensor_data.device_summary (
    device_id String,
    date Date,
    min_temperature Float64,
    max_temperature Float64,
    avg_temperature Float64,
    min_humidity Float64,
    max_humidity Float64,
    avg_humidity Float64,
    min_pressure Float64,
    max_pressure Float64,
    avg_pressure Float64,
    reading_count UInt32,
    last_seen DateTime
) ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (device_id, date)
TTL date + INTERVAL 365 DAY;

-- Materialized view - real-time aggregation için
CREATE MATERIALIZED VIEW IF NOT EXISTS sensor_data.device_summary_mv TO sensor_data.device_summary AS
SELECT 
    device_id,
    toDate(timestamp) as date,
    min(temperature) as min_temperature,
    max(temperature) as max_temperature,
    avg(temperature) as avg_temperature,
    min(humidity) as min_humidity,
    max(humidity) as max_humidity,
    avg(humidity) as avg_humidity,
    min(pressure) as min_pressure,
    max(pressure) as max_pressure,
    avg(pressure) as avg_pressure,
    count() as reading_count,
    max(timestamp) as last_seen
FROM sensor_data.sensor_readings
GROUP BY device_id, toDate(timestamp);

-- Alert log tablosu
CREATE TABLE IF NOT EXISTS sensor_data.alert_logs (
    alert_id String,
    device_id String,
    alert_type LowCardinality(String),
    message String,
    severity LowCardinality(String),
    temperature Float64,
    threshold Float64,
    timestamp DateTime64(3, 'UTC'),
    created_at DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (device_id, timestamp)
TTL timestamp + INTERVAL 30 DAY;

-- Performance için index'ler
CREATE INDEX IF NOT EXISTS idx_device_timestamp ON sensor_data.sensor_readings (device_id, timestamp) TYPE minmax GRANULARITY 1;
CREATE INDEX IF NOT EXISTS idx_temperature ON sensor_data.sensor_readings (temperature) TYPE minmax GRANULARITY 1;

-- User oluştur ve permissions ver
CREATE USER IF NOT EXISTS 'twinup' IDENTIFIED BY 'twinup123';
GRANT ALL ON sensor_data.* TO 'twinup';
