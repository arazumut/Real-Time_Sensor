# Real-Time Sensor Event Ingestion System

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Kafka](https://img.shields.io/badge/Apache%20Kafka-Streaming-231F20?style=flat&logo=apache-kafka)](https://kafka.apache.org/)
[![ClickHouse](https://img.shields.io/badge/ClickHouse-Database-FFCC01?style=flat&logo=clickhouse)](https://clickhouse.com/)
[![Prometheus](https://img.shields.io/badge/Prometheus-Monitoring-E6522C?style=flat&logo=prometheus)](https://prometheus.io/)

Modern IoT platformu için **gerçek zamanlı sensör verisi işleme sistemi**. Yüksek performanslı, ölçeklenebilir mikroservis mimarisi ile 10.000+ cihazdan gelen verileri düşük gecikmeyle işler.

## 📋 Özellikler

- 🏃‍♂️ **Yüksek Performans**: 10,000+ msg/s throughput
- 🔄 **Gerçek Zamanlı İşleme**: <100ms end-to-end latency
- 📊 **Kapsamlı Monitoring**: Prometheus + Grafana
- 🔒 **Güvenlik**: JWT authentication + role-based access
- 🐳 **Containerized**: Docker + Docker Compose
- 🧪 **Test Coverage**: %80+ unit test coverage
- 📈 **Ölçeklenebilir**: Horizontal scaling ready

## 🏗️ Sistem Mimarisi

```
┌─────────────┐    ┌─────────┐    ┌───────────────────┐    ┌─────────────┐
│   Sensörler │───▶│  Kafka  │───▶│   Mikroservisler  │───▶│ ClickHouse  │
│   (10K+)    │    │ Cluster │    │                   │    │   + Redis   │
└─────────────┘    └─────────┘    └───────────────────┘    └─────────────┘
                                           │
                                           ▼
                                  ┌─────────────────┐
                                  │ Prometheus +    │
                                  │ Grafana         │
                                  │ (Monitoring)    │
                                  └─────────────────┘
```

## 🚀 Hızlı Başlangıç

### Ön Gereksinimler

- **Go 1.21+**
- **Docker & Docker Compose**
- **Make** (opsiyonel, kolaylık için)

### 1. Projeyi Klonlayın

```bash
git clone <repository-url>
cd twinup-sensor-system
```

### 2. Bağımlılıkları Yükleyin

```bash
make deps
# veya
go mod download
```

### 3. Infrastructure'ı Başlatın

```bash
# Tüm servisleri başlat
make docker-up

# Veya sadece infrastructure
make docker-up-infra
```

### 4. Servisleri Test Edin

```bash
# Health check
make health-check

# Logs'ları izleyin
make docker-logs
```

## 📦 Mikroservisler

### 1. Sensor Producer Service
- **Port**: 8080 (metrics)
- **Amaç**: 10.000+ cihaz simülasyonu
- **Özellikler**: Concurrent processing, Kafka batch producer

### 2. Sensor Consumer Service  
- **Port**: 8080 (metrics)
- **Amaç**: Kafka consumer, ClickHouse writer, Redis cache
- **Özellikler**: Batch processing, gRPC client

### 3. Real-time Analytics Service
- **Port**: 8080 (metrics)  
- **Amaç**: Trend analizi, MQTT publisher
- **Özellikler**: Stream processing, MQTT QoS 1

### 4. Alert Handler Service
- **Port**: 50051 (gRPC), 8080 (metrics)
- **Amaç**: Temperature alerts, email notifications
- **Özellikler**: gRPC server, threshold monitoring

### 5. Authentication Service
- **Port**: 8080 (HTTP + metrics)
- **Amaç**: JWT authentication, role management
- **Özellikler**: JWT tokens, Redis caching

### 6. API Gateway Service
- **Port**: 8081 (HTTP)
- **Amaç**: REST API, data retrieval
- **Özellikler**: Redis cache, ClickHouse fallback

## 🔧 Konfigürasyon

### Environment Variables

```bash
# Kafka Configuration
TWINUP_KAFKA_BROKERS=localhost:9092
TWINUP_KAFKA_TOPIC=sensor-data

# Producer Configuration  
TWINUP_PRODUCER_DEVICE_COUNT=10000
TWINUP_PRODUCER_BATCH_SIZE=1000
TWINUP_PRODUCER_PRODUCE_INTERVAL=1s

# Database Configuration
TWINUP_CLICKHOUSE_HOST=localhost:9000
TWINUP_CLICKHOUSE_USER=twinup
TWINUP_CLICKHOUSE_PASSWORD=twinup123

# Redis Configuration
TWINUP_REDIS_HOST=localhost:6379

# Logging
TWINUP_LOGGER_LEVEL=info
TWINUP_LOGGER_FORMAT=json
```

### Configuration Files

Configuration dosyaları `configs/` klasöründe bulunur:

- `producer-config.yaml` - Producer service konfigürasyonu
- `consumer-config.yaml` - Consumer service konfigürasyonu  
- `analytics-config.yaml` - Analytics service konfigürasyonu

## 🧪 Testing

```bash
# Tüm testleri çalıştır
make test

# Coverage raporu oluştur
make test-coverage

# Benchmark testleri
make test-bench

# Sadece producer testleri
cd services/sensor-producer-service && go test ./...
```

## 📊 Monitoring & Observability

### Prometheus Metrics

Her servis kendi metrics'lerini expose eder:

- **Producer**: `http://localhost:8080/metrics`
- **Consumer**: `http://localhost:8080/metrics`  
- **Analytics**: `http://localhost:8080/metrics`

### Grafana Dashboards

Grafana'ya erişim: `http://localhost:3000`
- **Username**: admin
- **Password**: twinup123

### Key Metrics

- `sensor_producer_messages_produced_total` - Üretilen mesaj sayısı
- `sensor_producer_latency_ms` - Producer latency
- `sensor_consumer_lag` - Kafka consumer lag
- `db_insert_latency_ms` - Database insert latency

## 🐳 Docker Operations

```bash
# Tüm servisleri başlat
make docker-up

# Sadece infrastructure
make docker-up-infra  

# Logları izle
make docker-logs

# Servisleri durdur
make docker-down

# Volumes'ları da sil
make docker-down-volumes

# Service durumunu kontrol et
make status
```

## 🔍 Debugging & Troubleshooting

### Kafka Topics

```bash
# Topic'leri listele
make kafka-topics

# Sensor data topic'ini oluştur
make kafka-create-topic
```

### Database Access

```bash
# ClickHouse client
make clickhouse-client

# Redis CLI
make redis-cli
```

### Common Issues

1. **Kafka Connection Failed**
   ```bash
   # Kafka'nın hazır olup olmadığını kontrol edin
   docker-compose logs kafka
   ```

2. **High Memory Usage**
   ```bash
   # Container resource'larını kontrol edin
   docker stats
   ```

3. **Slow Performance**
   ```bash
   # Metrics'leri kontrol edin
   curl http://localhost:8080/metrics
   ```

## 📈 Performance Tuning

### Kafka Producer Optimization

```yaml
kafka:
  compression_type: "snappy"
  flush_frequency: "100ms"
  flush_messages: 1000
  flush_bytes: 1048576
```

### Device Simulation Tuning

```yaml
producer:
  device_count: 10000
  batch_size: 1000
  worker_count: 10
  buffer_size: 10000
```

## 🛠️ Development

### Local Development

```bash
# Infrastructure'ı başlat
make dev-setup

# Producer'ı local çalıştır
make run-producer

# Testleri çalıştır
make test
```

### Code Quality

```bash
# Format code
make fmt

# Run linter  
make lint

# Run vet
make vet

# All quality checks
make qa
```

## 📚 API Documentation

### REST API Endpoints

```
GET  /api/v1/devices/{id}/metrics    - Device metrics
GET  /api/v1/devices                 - List devices  
GET  /api/v1/health                  - Health check
POST /auth/login                     - Authentication
```

### gRPC Services

```
AlertService.SendAlert               - Send temperature alert
AlertService.GetAlertHistory         - Get alert history
AlertService.HealthCheck             - Service health
```

## 🔐 Security

- **JWT Authentication**: Tüm API endpoints protected
- **Role-based Access**: Admin, user, readonly roles
- **Rate Limiting**: API endpoint protection
- **Input Validation**: Tüm inputs validated

## 🚀 Deployment

### Production Deployment

```bash
# Production build
docker-compose -f docker-compose.prod.yml up -d

# Kubernetes deployment
kubectl apply -f deployments/k8s/
```

### Scaling

```bash
# Scale consumer service
docker-compose up --scale sensor-consumer=3

# Scale analytics service  
docker-compose up --scale real-time-analytics=2
```

## 📞 Support & Contributing

- **Issues**: GitHub Issues
- **Documentation**: `/docs` klasörü
- **Contributing**: CONTRIBUTING.md

## 📄 License

Bu proje TWINUP tarafından geliştirilmiştir.

---

**🦁 TWINUP Sensor System - Mükemmel IoT Çözümü! 🚀**
