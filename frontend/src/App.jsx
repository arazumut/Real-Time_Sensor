import { BrowserRouter as Router, Routes, Route, Link } from 'react-router-dom';
import { useState, useEffect } from 'react';
import './App.css';

// Pages
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import DeviceList from './pages/DeviceList';
import DeviceMetrics from './pages/DeviceMetrics';
import DeviceAnalytics from './pages/DeviceAnalytics';
import DeviceHistory from './pages/DeviceHistory';
import SystemStats from './pages/SystemStats';
import AdminPanel from './pages/AdminPanel';
import Profile from './pages/Profile';

function App() {
  const [token, setToken] = useState(localStorage.getItem('token'));
  const [user, setUser] = useState(null);

  useEffect(() => {
    if (token) {
      // Token varsa user bilgilerini al
      fetchProfile();
    }
  }, [token]);

  const fetchProfile = async () => {
    try {
      const response = await fetch('http://localhost:8080/auth/profile', {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });
      if (response.ok) {
        const data = await response.json();
        setUser(data.user);
      } else {
        logout();
      }
    } catch (error) {
      console.error('Profile fetch error:', error);
      logout();
    }
  };

  const logout = () => {
    localStorage.removeItem('token');
    setToken(null);
    setUser(null);
  };

  if (!token) {
    return <Login setToken={setToken} />;
  }

  return (
    <Router>
      <div className="app">
        <nav className="navbar">
          <div className="nav-brand">
            <h2>Sensor System</h2>
          </div>
          <div className="nav-links">
            <Link to="/">Dashboard</Link>
            <Link to="/devices">Devices</Link>
            <Link to="/stats">System Stats</Link>
            <Link to="/profile">Profile ({user?.username})</Link>
            {user?.roles?.includes('admin') && (
              <Link to="/admin">Admin Panel</Link>
            )}
            <button onClick={logout} className="logout-btn">Logout</button>
          </div>
        </nav>

        <main className="main-content">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/devices" element={<DeviceList />} />
            <Route path="/device/:id/metrics" element={<DeviceMetrics />} />
            <Route path="/device/:id/analytics" element={<DeviceAnalytics />} />
            <Route path="/device/:id/history" element={<DeviceHistory />} />
            <Route path="/stats" element={<SystemStats />} />
            <Route path="/profile" element={<Profile user={user} token={token} />} />
            <Route path="/admin" element={<AdminPanel token={token} />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

export default App;