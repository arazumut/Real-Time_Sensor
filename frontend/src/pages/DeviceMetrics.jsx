import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import axios from 'axios';

const DeviceMetrics = () => {
  const { id } = useParams();
  const [metrics, setMetrics] = useState('');
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(false);

  useEffect(() => {
    fetchMetrics();
  }, [id]);

  useEffect(() => {
    let interval;
    if (autoRefresh) {
      interval = setInterval(fetchMetrics, 5000); // 5 saniyede bir
    }
    return () => clearInterval(interval);
  }, [autoRefresh, id]);

  const fetchMetrics = async () => {
    setLoading(true);
    try {
      const response = await axios.get(`http://localhost:8081/api/v1/devices/${id}/metrics`);
      setMetrics(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setMetrics(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const checkAlert = (metricsData) => {
    try {
      const data = JSON.parse(metricsData);
      if (data.temperature > 90) {
        return '🔥 HIGH TEMPERATURE ALERT! > 90°C';
      } else if (data.temperature > 80) {
        return '⚠️ Warning: Temperature > 80°C';
      } else {
        return '✅ Temperature Normal';
      }
    } catch {
      return 'No temperature data';
    }
  };

  return (
    <div className="page">
      <h1>📊 Device Metrics: {id}</h1>
      
      <div className="grid">
        <div className="form-group">
          <button onClick={fetchMetrics} className="btn" disabled={loading}>
            {loading ? 'Loading...' : '🔄 Refresh Metrics'}
          </button>
        </div>
        
        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            🔄 Auto Refresh (5s)
          </label>
        </div>
      </div>

      <div className="form-group">
        <h3>🌡️ Alert Status</h3>
        <div className={`status-${metrics.includes('temperature') ? 'healthy' : 'warning'}`}>
          {checkAlert(metrics)}
        </div>
      </div>

      <div className="form-group">
        <h3>📊 Current Metrics</h3>
        <textarea
          className="result-area"
          value={metrics}
          readOnly
          placeholder="Device metrics will appear here..."
        />
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`📊 Device Metrics API:

Endpoint: GET /api/v1/devices/{id}/metrics
Base URL: http://localhost:8081
Current Device: ${id}

Description:
Gets current sensor readings for the specified device.

Response Format:
{
  "device_id": "${id}",
  "timestamp": "2025-09-27T10:00:00Z",
  "temperature": 25.5,
  "humidity": 60.2,
  "pressure": 1013.25,
  "location": {
    "lat": 41.0082,
    "lng": 28.9784
  },
  "status": "active"
}

Alert Thresholds:
• Temperature > 90°C: 🔥 CRITICAL ALERT
• Temperature > 80°C: ⚠️ WARNING
• Temperature ≤ 80°C: ✅ NORMAL`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>
    </div>
  );
};

export default DeviceMetrics;
