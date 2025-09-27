import { useState } from 'react';
import axios from 'axios';

const Profile = ({ user, token }) => {
  const [profileData, setProfileData] = useState('');
  const [tokenVerification, setTokenVerification] = useState('');
  const [loading, setLoading] = useState(false);

  const fetchProfile = async () => {
    setLoading(true);
    try {
      const response = await axios.get('http://localhost:8080/auth/profile', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      setProfileData(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setProfileData(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const verifyToken = async () => {
    try {
      const response = await axios.post('http://localhost:8080/auth/verify', {}, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      setTokenVerification(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setTokenVerification(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    }
  };

  const refreshToken = async () => {
    try {
      const refreshToken = localStorage.getItem('refresh_token');
      if (!refreshToken) {
        setTokenVerification('No refresh token available');
        return;
      }

      const response = await axios.post('http://localhost:8080/auth/refresh', {
        refresh_token: refreshToken
      });
      
      if (response.data.access_token) {
        localStorage.setItem('token', response.data.access_token);
        setTokenVerification(JSON.stringify(response.data, null, 2));
      }
    } catch (err) {
      setTokenVerification(`Refresh Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    }
  };

  return (
    <div className="page">
      <h1>👤 User Profile</h1>
      
      <div className="form-group">
        <h3>👤 Current User Info</h3>
        <textarea
          className="result-area"
          value={user ? JSON.stringify(user, null, 2) : 'No user data available'}
          readOnly
          style={{ minHeight: '200px' }}
        />
      </div>

      <div className="form-group">
        <button onClick={fetchProfile} className="btn" disabled={loading}>
          {loading ? 'Loading...' : '🔄 Fetch Profile'}
        </button>
        <button onClick={verifyToken} className="btn btn-success">
          ✅ Verify Token
        </button>
        <button onClick={refreshToken} className="btn">
          🔄 Refresh Token
        </button>
      </div>

      <div className="form-group">
        <h3>📊 Profile API Response</h3>
        <textarea
          className="result-area"
          value={profileData}
          readOnly
          placeholder="Profile data will appear here..."
        />
      </div>

      <div className="form-group">
        <h3>🔐 Token Verification</h3>
        <textarea
          className="result-area"
          value={tokenVerification}
          readOnly
          placeholder="Token verification results will appear here..."
        />
      </div>

      <div className="form-group">
        <h3>🔑 JWT Token Info</h3>
        <textarea
          className="result-area"
          value={`🔐 Current JWT Token:

Token: ${token ? token.substring(0, 50) + '...' : 'No token'}
Length: ${token ? token.length : 0} characters

🛡️ Token Structure:
• Header: Algorithm and token type
• Payload: User claims and permissions
• Signature: Security verification

🕐 Token Lifecycle:
• Access Token TTL: 15 minutes
• Refresh Token TTL: 24 hours
• Auto refresh available

🔒 Security Features:
• JWT HS256 algorithm
• Role-based permissions
• Session management
• Secure logout`}
          readOnly
          style={{ minHeight: '250px' }}
        />
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`👤 Profile & Auth APIs:

🔍 GET /auth/profile
• Description: Get current user profile
• Auth: Bearer token required
• Response: User info with roles and permissions

✅ POST /auth/verify  
• Description: Verify JWT token validity
• Auth: Bearer token required
• Response: Token validation status

🔄 POST /auth/refresh
• Description: Refresh access token
• Body: { "refresh_token": "..." }
• Response: New access token

🚪 POST /auth/logout
• Description: Logout and invalidate token
• Auth: Bearer token required
• Response: Logout confirmation`}
          readOnly
          style={{ minHeight: '250px' }}
        />
      </div>
    </div>
  );
};

export default Profile;
