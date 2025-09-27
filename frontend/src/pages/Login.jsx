import { useState } from 'react';
import axios from 'axios';

const Login = ({ setToken }) => {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('admin123');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [response, setResponse] = useState('');

  const handleLogin = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    setResponse('');

    try {
      const result = await axios.post('http://localhost:8080/auth/login', {
        username,
        password
      });

      setResponse(JSON.stringify(result.data, null, 2));
      
      if (result.data.success && result.data.access_token) {
        localStorage.setItem('token', result.data.access_token);
        setToken(result.data.access_token);
      }
    } catch (err) {
      setError(err.response?.data?.message || 'Login failed');
      setResponse(JSON.stringify(err.response?.data || { error: err.message }, null, 2));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-container">
      <div className="login-form">
        <h1 className="login-title">🦁 TWINUP Login</h1>
        
        <form onSubmit={handleLogin}>
          <div className="form-group">
            <label>Username:</label>
            <input
              type="text"
              className="form-input"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </div>
          
          <div className="form-group">
            <label>Password:</label>
            <input
              type="password"
              className="form-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          
          <button type="submit" className="btn btn-success" disabled={loading}>
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>

        {error && <div className="error">❌ {error}</div>}
        
        <div className="form-group">
          <label>🚀 API Response:</label>
          <textarea
            className="result-area"
            value={response}
            readOnly
            placeholder="Login response will appear here..."
          />
        </div>

        <div className="form-group">
          <h3>📋 Test Credentials:</h3>
          <textarea
            className="result-area"
            value={`Available Users:
• admin / admin123 (Admin role)
• user1 / user123 (User role)  
• operator1 / operator123 (Operator role)
• readonly1 / readonly123 (Readonly role)`}
            readOnly
            style={{ minHeight: '120px' }}
          />
        </div>
      </div>
    </div>
  );
};

export default Login;
