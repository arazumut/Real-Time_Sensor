import { useState, useEffect } from 'react';
import axios from 'axios';

const AdminPanel = ({ token }) => {
  const [roles, setRoles] = useState('');
  const [authMetrics, setAuthMetrics] = useState('');
  const [gatewayMetrics, setGatewayMetrics] = useState('');
  const [prometheusTargets, setPrometheusTargets] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchAdminData();
  }, []);

  const fetchAdminData = async () => {
    setLoading(true);

    // Fetch Roles
    try {
      const rolesResponse = await axios.get('http://localhost:8080/auth/admin/roles', {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      setRoles(JSON.stringify(rolesResponse.data, null, 2));
    } catch (err) {
      setRoles(`Error: ${err.response?.data?.message || err.message}`);
    }

    // Fetch Auth Metrics
    try {
      const authMetricsResponse = await axios.get('http://localhost:9080/metrics');
      setAuthMetrics(authMetricsResponse.data);
    } catch (err) {
      setAuthMetrics(`Error: ${err.message}`);
    }

    // Fetch Gateway Metrics
    try {
      const gatewayMetricsResponse = await axios.get('http://localhost:9081/metrics');
      setGatewayMetrics(gatewayMetricsResponse.data);
    } catch (err) {
      setGatewayMetrics(`Error: ${err.message}`);
    }

    // Fetch Prometheus Targets
    try {
      const targetsResponse = await axios.get('http://localhost:9090/api/v1/targets');
      setPrometheusTargets(JSON.stringify(targetsResponse.data, null, 2));
    } catch (err) {
      setPrometheusTargets(`Error: ${err.message}`);
    }

    setLoading(false);
  };

  return (
    <div className="page">
      <h1>👑 Admin Panel</h1>
      
      <div className="form-group">
        <button onClick={fetchAdminData} className="btn btn-success" disabled={loading}>
          {loading ? 'Loading...' : '🔄 Refresh All Admin Data'}
        </button>
      </div>

      <div className="form-group">
        <h3>👥 RBAC Roles & Permissions</h3>
        <textarea
          className="result-area"
          value={roles}
          readOnly
          placeholder="Roles and permissions will appear here..."
          style={{ minHeight: '300px' }}
        />
      </div>

      <div className="grid">
        <div>
          <h3>📊 Authentication Metrics</h3>
          <textarea
            className="result-area"
            value={authMetrics}
            readOnly
            placeholder="Auth service metrics will appear here..."
            style={{ minHeight: '300px' }}
          />
        </div>

        <div>
          <h3>🌐 Gateway Metrics</h3>
          <textarea
            className="result-area"
            value={gatewayMetrics}
            readOnly
            placeholder="Gateway service metrics will appear here..."
            style={{ minHeight: '300px' }}
          />
        </div>
      </div>

      <div className="form-group">
        <h3>🎯 Prometheus Targets</h3>
        <textarea
          className="result-area"
          value={prometheusTargets}
          readOnly
          placeholder="Prometheus targets will appear here..."
          style={{ minHeight: '400px' }}
        />
      </div>

      <div className="form-group">
        <h3>🔗 Admin Links</h3>
        <div className="grid">
          <div>
            <h4>📊 Monitoring</h4>
            <a href="http://localhost:9090" target="_blank" rel="noopener noreferrer" className="device-link">
              🔗 Prometheus (Port 9090)
            </a><br/>
            <a href="http://localhost:3000" target="_blank" rel="noopener noreferrer" className="device-link">
              🔗 Grafana (Port 3000)
            </a>
          </div>
          <div>
            <h4>🗄️ Database</h4>
            <a href="http://localhost:8123" target="_blank" rel="noopener noreferrer" className="device-link">
              🔗 ClickHouse (Port 8123)
            </a><br/>
            <span className="device-link">🔴 Redis (Port 6379)</span>
          </div>
        </div>
      </div>

      <div className="form-group">
        <h3>📖 Admin API Documentation</h3>
        <textarea
          className="result-area"
          value={`👑 Admin Panel APIs:

🔐 GET /auth/admin/roles
• Description: Get all system roles
• Auth: Admin role required
• Response: Complete RBAC configuration

📊 GET /metrics (Port 9080)
• Description: Authentication service metrics
• Format: Prometheus metrics
• Includes: Login stats, cache metrics, performance

🌐 GET /metrics (Port 9081)  
• Description: API Gateway service metrics
• Format: Prometheus metrics
• Includes: Request stats, cache hits, database metrics

🎯 GET /api/v1/targets (Prometheus)
• Description: All monitored service targets
• URL: http://localhost:9090/api/v1/targets
• Response: Service health and scrape status

🔍 GET /api/v1/query (Prometheus)
• Description: Execute PromQL queries
• URL: http://localhost:9090/api/v1/query?query=up
• Response: Metric query results`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>
    </div>
  );
};

export default AdminPanel;
