import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import axios from 'axios';

const DeviceList = () => {
  const [devices, setDevices] = useState('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchDevices();
  }, [page, pageSize]);

  const fetchDevices = async () => {
    setLoading(true);
    try {
      const response = await axios.get(`http://localhost:8081/api/v1/devices?page=${page}&page_size=${pageSize}`);
      setDevices(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setDevices(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const testDeviceIds = ['sensor-001', 'sensor-002', 'test_device_001'];

  return (
    <div className="page">
      <h1>📱 Device List</h1>
      
      <div className="grid">
        <div className="form-group">
          <label>Page:</label>
          <input
            type="number"
            className="form-input"
            value={page}
            onChange={(e) => setPage(parseInt(e.target.value))}
            min="1"
          />
        </div>
        
        <div className="form-group">
          <label>Page Size:</label>
          <input
            type="number"
            className="form-input"
            value={pageSize}
            onChange={(e) => setPageSize(parseInt(e.target.value))}
            min="1"
            max="1000"
          />
        </div>
      </div>

      <div className="form-group">
        <button onClick={fetchDevices} className="btn" disabled={loading}>
          {loading ? 'Loading...' : '🔄 Fetch Devices'}
        </button>
      </div>

      <div className="form-group">
        <h3>📋 Device List Response</h3>
        <textarea
          className="result-area"
          value={devices}
          readOnly
          placeholder="Device list will appear here..."
        />
      </div>

      <div className="form-group">
        <h3>🧪 Test Device Links</h3>
        <div className="grid">
          {testDeviceIds.map(deviceId => (
            <div key={deviceId} className="device-card">
              <h3>📟 {deviceId}</h3>
              <div>
                <Link to={`/device/${deviceId}/metrics`} className="device-link">
                  📊 Metrics
                </Link>
                <Link to={`/device/${deviceId}/analytics`} className="device-link">
                  📈 Analytics
                </Link>
                <Link to={`/device/${deviceId}/history`} className="device-link">
                  📚 History
                </Link>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`🌐 Device List API:

Endpoint: GET /api/v1/devices
Base URL: http://localhost:8081

Query Parameters:
• page: Page number (default: 1)
• page_size: Items per page (default: 100, max: 1000)

Example URLs:
• http://localhost:8081/api/v1/devices
• http://localhost:8081/api/v1/devices?page=1&page_size=10
• http://localhost:8081/api/v1/devices?page=2&page_size=50

Response Format:
{
  "success": true,
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 10,
    "total": 100
  }
}`}
          readOnly
          style={{ minHeight: '250px' }}
        />
      </div>
    </div>
  );
};

export default DeviceList;
