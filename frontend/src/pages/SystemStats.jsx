import { useState, useEffect } from 'react';
import axios from 'axios';

const SystemStats = () => {
  const [stats, setStats] = useState('');
  const [prometheusQuery, setPrometheusQuery] = useState('up');
  const [prometheusResult, setPrometheusResult] = useState('');
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);

  useEffect(() => {
    fetchStats();
  }, []);

  useEffect(() => {
    let interval;
    if (autoRefresh) {
      interval = setInterval(fetchStats, 10000); // 10 saniyede bir
    }
    return () => clearInterval(interval);
  }, [autoRefresh]);

  const fetchStats = async () => {
    setLoading(true);
    try {
      const response = await axios.get('http://localhost:8081/api/v1/stats');
      setStats(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setStats(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const queryPrometheus = async () => {
    try {
      const response = await axios.get(`http://localhost:9090/api/v1/query?query=${encodeURIComponent(prometheusQuery)}`);
      setPrometheusResult(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setPrometheusResult(`Error: ${err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    }
  };

  const commonQueries = [
    { label: '🔄 Service Status', query: 'up' },
    { label: '📊 Auth Metrics', query: 'auth_http_requests_total' },
    { label: '🌐 Gateway Metrics', query: 'gateway_http_requests_total' },
    { label: '💾 Memory Usage', query: 'go_memstats_alloc_bytes' },
    { label: '🔥 CPU Usage', query: 'process_cpu_seconds_total' }
  ];

  return (
    <div className="page">
      <h1>📊 System Statistics</h1>
      
      <div className="grid">
        <div className="form-group">
          <button onClick={fetchStats} className="btn" disabled={loading}>
            {loading ? 'Loading...' : '🔄 Refresh Stats'}
          </button>
        </div>
        
        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            🔄 Auto Refresh (10s)
          </label>
        </div>
      </div>

      <div className="form-group">
        <h3>📊 System Statistics</h3>
        <textarea
          className="result-area"
          value={stats}
          readOnly
          placeholder="System statistics will appear here..."
          style={{ minHeight: '400px' }}
        />
      </div>

      <div className="form-group">
        <h3>📊 Prometheus Queries</h3>
        <div className="grid">
          <div className="form-group">
            <label>PromQL Query:</label>
            <input
              type="text"
              className="form-input"
              value={prometheusQuery}
              onChange={(e) => setPrometheusQuery(e.target.value)}
              placeholder="Enter PromQL query..."
            />
          </div>
          <div className="form-group">
            <button onClick={queryPrometheus} className="btn btn-success">
              🔍 Execute Query
            </button>
          </div>
        </div>
        
        <div className="form-group">
          <h4>🔥 Common Queries:</h4>
          {commonQueries.map(q => (
            <button
              key={q.query}
              onClick={() => setPrometheusQuery(q.query)}
              className="btn"
              style={{ margin: '0.25rem' }}
            >
              {q.label}
            </button>
          ))}
        </div>
        
        <textarea
          className="result-area"
          value={prometheusResult}
          readOnly
          placeholder="Prometheus query results will appear here..."
          style={{ minHeight: '300px' }}
        />
      </div>

      <div className="form-group">
        <h3>🔗 External Services</h3>
        <div className="grid">
          <div>
            <h4>📊 Prometheus</h4>
            <a href="http://localhost:9090" target="_blank" rel="noopener noreferrer" className="device-link">
              🔗 Open Prometheus UI
            </a>
          </div>
          <div>
            <h4>📈 Grafana</h4>
            <a href="http://localhost:3000" target="_blank" rel="noopener noreferrer" className="device-link">
              🔗 Open Grafana Dashboards
            </a>
            <p>Username: admin, Password: twinup123</p>
          </div>
        </div>
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`📊 System Stats API:

Endpoint: GET /api/v1/stats
Base URL: http://localhost:8081

Description:
Provides comprehensive system statistics including:
• Cache performance metrics
• Database connection status  
• Gateway uptime and version
• Service health indicators

Response Format:
{
  "success": true,
  "data": {
    "cache": {
      "connection_active": true,
      "hit_rate_pct": 75.5,
      "total_ops": 1000,
      "uptime_seconds": 3600
    },
    "database": {
      "connection_active": true,
      "query_count": 500,
      "error_count": 0
    },
    "gateway": {
      "version": "1.0.0",
      "uptime": "1h30m",
      "timestamp": "2025-09-27T10:00:00Z"
    }
  }
}`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>
    </div>
  );
};

export default SystemStats;
