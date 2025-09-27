import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import axios from 'axios';

const DeviceHistory = () => {
  const { id } = useParams();
  const [history, setHistory] = useState('');
  const [startTime, setStartTime] = useState('2025-09-27T10:00:00Z');
  const [endTime, setEndTime] = useState('2025-09-27T14:00:00Z');
  const [limit, setLimit] = useState(100);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    fetchHistory();
  }, [id]);

  const fetchHistory = async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        start_time: startTime,
        end_time: endTime,
        limit: limit.toString()
      });
      
      const response = await axios.get(`http://localhost:8081/api/v1/devices/${id}/history?${params}`);
      setHistory(JSON.stringify(response.data, null, 2));
    } catch (err) {
      setHistory(`Error: ${err.response?.data?.message || err.message}\n\n${JSON.stringify(err.response?.data || { error: err.message }, null, 2)}`);
    } finally {
      setLoading(false);
    }
  };

  const setQuickTimeRange = (hours) => {
    const now = new Date();
    const start = new Date(now.getTime() - hours * 60 * 60 * 1000);
    setStartTime(start.toISOString());
    setEndTime(now.toISOString());
  };

  return (
    <div className="page">
      <h1>📚 Device History: {id}</h1>
      
      <div className="grid">
        <div className="form-group">
          <label>Start Time:</label>
          <input
            type="datetime-local"
            className="form-input"
            value={startTime.slice(0, -1)}
            onChange={(e) => setStartTime(e.target.value + 'Z')}
          />
        </div>
        
        <div className="form-group">
          <label>End Time:</label>
          <input
            type="datetime-local"
            className="form-input"
            value={endTime.slice(0, -1)}
            onChange={(e) => setEndTime(e.target.value + 'Z')}
          />
        </div>
        
        <div className="form-group">
          <label>Limit:</label>
          <input
            type="number"
            className="form-input"
            value={limit}
            onChange={(e) => setLimit(parseInt(e.target.value))}
            min="1"
            max="10000"
          />
        </div>
      </div>

      <div className="form-group">
        <h3>⏰ Quick Time Ranges</h3>
        <button onClick={() => setQuickTimeRange(1)} className="btn">Last 1 Hour</button>
        <button onClick={() => setQuickTimeRange(6)} className="btn">Last 6 Hours</button>
        <button onClick={() => setQuickTimeRange(24)} className="btn">Last 24 Hours</button>
        <button onClick={() => setQuickTimeRange(168)} className="btn">Last Week</button>
      </div>

      <div className="form-group">
        <button onClick={fetchHistory} className="btn btn-success" disabled={loading}>
          {loading ? 'Loading...' : '🔄 Fetch History'}
        </button>
      </div>

      <div className="form-group">
        <h3>📚 Historical Data</h3>
        <textarea
          className="result-area"
          value={history}
          readOnly
          placeholder="Device history will appear here..."
          style={{ minHeight: '400px' }}
        />
      </div>

      <div className="form-group">
        <h3>📖 API Documentation</h3>
        <textarea
          className="result-area"
          value={`📚 Device History API:

Endpoint: GET /api/v1/devices/{id}/history
Base URL: http://localhost:8081
Current Device: ${id}

Query Parameters:
• start_time: Start time in RFC3339 format
• end_time: End time in RFC3339 format  
• limit: Maximum number of records (default: 1000)

Current Query:
start_time=${startTime}
end_time=${endTime}
limit=${limit}

Example URL:
http://localhost:8081/api/v1/devices/${id}/history?start_time=${startTime}&end_time=${endTime}&limit=${limit}

Response Format:
{
  "success": true,
  "data": [
    {
      "timestamp": "2025-09-27T10:00:00Z",
      "temperature": 25.5,
      "humidity": 60.2,
      "pressure": 1013.25
    }
  ],
  "count": 100
}`}
          readOnly
          style={{ minHeight: '300px' }}
        />
      </div>
    </div>
  );
};

export default DeviceHistory;
