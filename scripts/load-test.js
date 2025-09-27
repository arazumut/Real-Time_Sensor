// TWINUP API Gateway Load Testing Script
// K6 load testing for API Gateway performance validation

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const responseTime = new Trend('response_time');
const requestCount = new Counter('requests');

// Test configuration
export let options = {
  stages: [
    // Warm-up
    { duration: '30s', target: 10 },
    
    // Ramp-up to normal load
    { duration: '1m', target: 50 },
    
    // Normal load
    { duration: '2m', target: 50 },
    
    // Peak load
    { duration: '1m', target: 100 },
    
    // Stress test
    { duration: '30s', target: 200 },
    
    // Ramp-down
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    http_req_failed: ['rate<0.1'],    // Error rate under 10%
    errors: ['rate<0.1'],             // Custom error rate under 10%
  },
};

// Base URL configuration
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';

// Test data
const DEVICE_IDS = [
  'temperature_sensor_000001',
  'temperature_sensor_000002', 
  'humidity_sensor_000001',
  'weather_station_000001',
  'industrial_sensor_000001',
  'pressure_sensor_000001',
];

export default function() {
  // Test device metrics endpoint
  group('Device Metrics API', function() {
    const deviceId = DEVICE_IDS[Math.floor(Math.random() * DEVICE_IDS.length)];
    const url = `${BASE_URL}/api/v1/devices/${deviceId}/metrics`;
    
    const response = http.get(url, {
      headers: {
        'Accept': 'application/json',
      },
    });
    
    requestCount.add(1);
    responseTime.add(response.timings.duration);
    
    const success = check(response, {
      'device metrics status is 200 or 404': (r) => r.status === 200 || r.status === 404,
      'device metrics response time < 500ms': (r) => r.timings.duration < 500,
      'device metrics has valid JSON': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body !== null;
        } catch (e) {
          return false;
        }
      },
    });
    
    if (!success) {
      errorRate.add(1);
    }
  });

  // Test device list endpoint
  group('Device List API', function() {
    const page = Math.floor(Math.random() * 5) + 1;
    const pageSize = 50;
    const url = `${BASE_URL}/api/v1/devices?page=${page}&page_size=${pageSize}`;
    
    const response = http.get(url, {
      headers: {
        'Accept': 'application/json',
      },
    });
    
    requestCount.add(1);
    responseTime.add(response.timings.duration);
    
    const success = check(response, {
      'device list status is 200': (r) => r.status === 200,
      'device list response time < 1000ms': (r) => r.timings.duration < 1000,
      'device list has pagination': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.data && typeof body.data.page === 'number';
        } catch (e) {
          return false;
        }
      },
    });
    
    if (!success) {
      errorRate.add(1);
    }
  });

  // Test device analytics endpoint
  group('Device Analytics API', function() {
    const deviceId = DEVICE_IDS[Math.floor(Math.random() * DEVICE_IDS.length)];
    const analyticsTypes = ['trend', 'delta', 'regional', 'all'];
    const type = analyticsTypes[Math.floor(Math.random() * analyticsTypes.length)];
    const url = `${BASE_URL}/api/v1/devices/${deviceId}/analytics?type=${type}`;
    
    const response = http.get(url, {
      headers: {
        'Accept': 'application/json',
      },
    });
    
    requestCount.add(1);
    responseTime.add(response.timings.duration);
    
    const success = check(response, {
      'analytics status is 200': (r) => r.status === 200,
      'analytics response time < 300ms': (r) => r.timings.duration < 300,
      'analytics has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.data && body.data.analytics_type;
        } catch (e) {
          return false;
        }
      },
    });
    
    if (!success) {
      errorRate.add(1);
    }
  });

  // Test health check endpoint
  group('Health Check API', function() {
    const url = `${BASE_URL}/api/v1/health`;
    
    const response = http.get(url);
    
    requestCount.add(1);
    
    const success = check(response, {
      'health check status is 200': (r) => r.status === 200,
      'health check response time < 100ms': (r) => r.timings.duration < 100,
      'health check has status': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.status !== undefined;
        } catch (e) {
          return false;
        }
      },
    });
    
    if (!success) {
      errorRate.add(1);
    }
  });

  // Test system stats endpoint
  group('System Stats API', function() {
    const url = `${BASE_URL}/api/v1/stats`;
    
    const response = http.get(url);
    
    requestCount.add(1);
    
    const success = check(response, {
      'stats status is 200': (r) => r.status === 200,
      'stats response time < 200ms': (r) => r.timings.duration < 200,
      'stats has data': (r) => {
        try {
          const body = JSON.parse(r.body);
          return body.data && body.data.gateway;
        } catch (e) {
          return false;
        }
      },
    });
    
    if (!success) {
      errorRate.add(1);
    }
  });

  // Random sleep between 0.5-2 seconds to simulate real user behavior
  sleep(Math.random() * 1.5 + 0.5);
}

// Setup function - runs once before the test
export function setup() {
  console.log('🚀 Starting TWINUP API Gateway Load Test');
  console.log(`📍 Target URL: ${BASE_URL}`);
  console.log(`📊 Test Duration: ~6 minutes`);
  console.log(`👥 Max VUs: 200`);
  console.log(`🎯 Performance Targets:`);
  console.log(`   - 95% requests < 500ms`);
  console.log(`   - Error rate < 10%`);
  console.log(`   - Health checks < 100ms`);
  
  // Test base connectivity
  const healthResponse = http.get(`${BASE_URL}/api/v1/health`);
  if (healthResponse.status !== 200) {
    console.error('❌ Health check failed - service may be down');
    throw new Error('Service health check failed');
  }
  
  console.log('✅ Pre-test health check passed');
  return { baseUrl: BASE_URL };
}

// Teardown function - runs once after the test
export function teardown(data) {
  console.log('🏁 Load test completed');
  console.log(`📍 Tested URL: ${data.baseUrl}`);
  
  // Final health check
  const healthResponse = http.get(`${data.baseUrl}/api/v1/health`);
  if (healthResponse.status === 200) {
    console.log('✅ Post-test health check passed');
  } else {
    console.log('⚠️ Post-test health check failed');
  }
}
