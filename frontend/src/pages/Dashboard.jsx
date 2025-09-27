import { useState, useEffect } from 'react';
import axios from 'axios';

const Dashboard = () => {
  const [authHealth, setAuthHealth] = useState('');
  const [gatewayHealth, setGatewayHealth] = useState('');
  const [systemStats, setSystemStats] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const fetchDashboardData = async () => {
    setLoading(true);
    
    // Auth Service Health
    try {
      const authResponse = await axios.get('http://localhost:8080/auth/health');
      setAuthHealth(JSON.stringify(authResponse.data, null, 2));
    } catch (err) {
      setAuthHealth(`Error: ${err.message}`);
    }

    // API Gateway Health  
    try {
      const gatewayResponse = await axios.get('http://localhost:8081/api/v1/health');
      setGatewayHealth(JSON.stringify(gatewayResponse.data, null, 2));
    } catch (err) {
      setGatewayHealth(`Error: ${err.message}`);
    }

    // System Stats
    try {
      const statsResponse = await axios.get('http://localhost:8081/api/v1/stats');
      setSystemStats(JSON.stringify(statsResponse.data, null, 2));
    } catch (err) {
      setSystemStats(`Error: ${err.message}`);
    }

    setLoading(false);
  };

  return (
    <div className="page">
      <h1>🏠 Dashboard</h1>
      
      <div className="form-group">
        <button onClick={fetchDashboardData} className="btn" disabled={loading}>
          {loading ? 'Refreshing...' : '🔄 Refresh Dashboard'}
        </button>
      </div>

      <div className="grid">
        <div>
          <h3>🔐 Authentication Service Health</h3>
          <textarea
            className="result-area"
            value={authHealth}
            readOnly
            placeholder="Authentication service health will appear here..."
          />
        </div>

        <div>
          <h3>🌐 API Gateway Health</h3>
          <textarea
            className="result-area"
            value={gatewayHealth}
            readOnly
            placeholder="API Gateway health will appear here..."
          />
        </div>
      </div>

      <div className="form-group">
        <h3>📊 System Statistics</h3>
        <textarea
          className="result-area"
          value={systemStats}
          readOnly
          placeholder="System statistics will appear here..."
          style={{ minHeight: '400px' }}
        />
      </div>

      <div className="form-group">
        <h3>📋 Available Services</h3>
        <textarea
          className="result-area"
          value={`🚀 TWINUP Sensor System Services:

🔐 Authentication Service (Port 8080)
   - Login/Logout
   - JWT Token Management
   - Role-Based Access Control
   - User Profile Management

🌐 API Gateway Service (Port 8081)  
   - Device Metrics
   - Device Analytics
   - Device History
   - System Statistics

📊 Monitoring Services
   - Prometheus (Port 9090)
   - Grafana (Port 3000)
   - ClickHouse (Port 8123)
   - Redis (Port 6379)

🚨 Alert Handler (gRPC Port 50051)
   - Temperature Alerts
   - Email Notifications
   - Alert History`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>
    </div>
  );
};

export default Dashboard;
