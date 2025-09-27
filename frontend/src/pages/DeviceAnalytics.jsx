import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import axios from 'axios';

const DeviceAnalytics = () => {
  const { id } = useParams();
  const [analytics, setAnalytics] = useState('');
  const [analyticsType, setAnalyticsType] = useState('all');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchAnalytics();
  }, [id, analyticsType]);

  const fetchAnalytics = async () => {
    setLoading(true);
    try {
      const response = await axios.get(`http://localhost:8081/api/v1/devices/${id}/analytics?type=${analyticsType}`);
      setAnalytics(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setAnalytics(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const analyticsTypes = [
    { value: 'all', label: '🔄 All Analytics' },
    { value: 'trend', label: '📈 Trend Analysis' },
    { value: 'delta', label: '📊 Delta Analysis' },
    { value: 'regional', label: '🗺️ Regional Analysis' }
  ];

  return (
    <div className="page">
      <h1>📈 Device Analytics: {id}</h1>
      
      <div className="grid">
        <div className="form-group">
          <label>Analytics Type:</label>
          <select
            className="form-input"
            value={analyticsType}
            onChange={(e) => setAnalyticsType(e.target.value)}
          >
            {analyticsTypes.map(type => (
              <option key={type.value} value={type.value}>
                {type.label}
              </option>
            ))}
          </select>
        </div>
        
        <div className="form-group">
          <button onClick={fetchAnalytics} className="btn" disabled={loading}>
            {loading ? 'Loading...' : '🔄 Refresh Analytics'}
          </button>
        </div>
      </div>

      <div className="form-group">
        <h3>📈 Analytics Results</h3>
        <textarea
          className="result-area"
          value={analytics}
          readOnly
          placeholder="Device analytics will appear here..."
          style={{ minHeight: '400px' }}
        />
      </div>

      <div className="form-group">
        <h3>🧠 Real-time Analytics Info</h3>
        <textarea
          className="result-area"
          value={`🚀 Real-time Analytics Service:

Current Analysis Type: ${analyticsType}
Device ID: ${id}

📊 Available Analytics:

1. 📈 TREND ANALYSIS
   - Direction: increasing/decreasing/stable
   - Slope: Rate of change
   - Strength: Confidence level (0-1)

2. 📊 DELTA ANALYSIS  
   - Change: Absolute change from previous
   - Percentage: Percentage change
   - Rate: Change per time unit

3. 🗺️ REGIONAL ANALYSIS
   - Average: Regional average
   - Rank: Device rank in region
   - Percentile: Regional percentile

4. 🔄 ALL ANALYTICS
   - Complete analysis package
   - All metrics combined
   - Comprehensive overview

🔄 Data Source: Real-time Analytics Service
📡 Protocol: MQTT QoS 1
⏱️ Update Frequency: Real-time
📊 Processing: Stream analytics`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`📈 Device Analytics API:

Endpoint: GET /api/v1/devices/{id}/analytics
Base URL: http://localhost:8081
Current Device: ${id}

Query Parameters:
• type: Analytics type (trend, delta, regional, all)
  - Default: all

Example URLs:
• http://localhost:8081/api/v1/devices/${id}/analytics
• http://localhost:8081/api/v1/devices/${id}/analytics?type=trend
• http://localhost:8081/api/v1/devices/${id}/analytics?type=delta`}
          readOnly
          style={{ minHeight: '200px' }}
        />
      </div>
    </div>
  );
};

export default DeviceAnalytics;
