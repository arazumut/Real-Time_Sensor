package simulator

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// DeviceType represents different types of IoT devices
type DeviceType string

const (
	TemperatureSensor DeviceType = "temperature_sensor"
	HumiditySensor    DeviceType = "humidity_sensor"
	PressureSensor    DeviceType = "pressure_sensor"
	WeatherStation    DeviceType = "weather_station"
	IndustrialSensor  DeviceType = "industrial_sensor"
)

// Location represents geographical coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Region    string  `json:"region"`
}

// SensorReading represents a single sensor reading
type SensorReading struct {
	DeviceID    string    `json:"device_id"`
	Timestamp   time.Time `json:"timestamp"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	Pressure    float64   `json:"pressure"`
	Location    Location  `json:"location"`
	Status      string    `json:"status"`
}

// Device represents a simulated IoT device
type Device struct {
	ID              string
	Type            DeviceType
	Location        Location
	BaseTemperature float64
	BaseHumidity    float64
	BasePressure    float64
	VariationFactor float64
	Status          string
	LastReading     time.Time
	ReadingCount    int64
	ErrorRate       float64
	mutex           sync.RWMutex
	random          *rand.Rand
}

// NewDevice creates a new simulated device
func NewDevice(id string, deviceType DeviceType, location Location) *Device {
	source := rand.NewSource(time.Now().UnixNano() + int64(len(id)))

	device := &Device{
		ID:       id,
		Type:     deviceType,
		Location: location,
		Status:   "active",
		random:   rand.New(source),
	}

	// Set base values based on device type and location
	device.setBaseValues()

	return device
}

// setBaseValues sets realistic base sensor values based on device type and location
func (d *Device) setBaseValues() {
	// Base values vary by region for realism
	regionMultiplier := d.getRegionMultiplier()

	switch d.Type {
	case TemperatureSensor:
		d.BaseTemperature = 20.0 + regionMultiplier*10
		d.BaseHumidity = 50.0
		d.BasePressure = 1013.25
		d.VariationFactor = 0.5
		d.ErrorRate = 0.001 // 0.1% error rate

	case HumiditySensor:
		d.BaseTemperature = 22.0 + regionMultiplier*8
		d.BaseHumidity = 45.0 + regionMultiplier*20
		d.BasePressure = 1013.25
		d.VariationFactor = 0.8
		d.ErrorRate = 0.002

	case PressureSensor:
		d.BaseTemperature = 21.0 + regionMultiplier*9
		d.BaseHumidity = 50.0
		d.BasePressure = 1013.25 + regionMultiplier*5
		d.VariationFactor = 0.3
		d.ErrorRate = 0.0005

	case WeatherStation:
		d.BaseTemperature = 18.0 + regionMultiplier*15
		d.BaseHumidity = 60.0 + regionMultiplier*15
		d.BasePressure = 1013.25 + regionMultiplier*3
		d.VariationFactor = 1.2
		d.ErrorRate = 0.0001

	case IndustrialSensor:
		d.BaseTemperature = 35.0 + regionMultiplier*25 // Industrial environments are hotter
		d.BaseHumidity = 40.0
		d.BasePressure = 1013.25
		d.VariationFactor = 2.0
		d.ErrorRate = 0.005 // Higher error rate in industrial environments

	default:
		d.BaseTemperature = 20.0
		d.BaseHumidity = 50.0
		d.BasePressure = 1013.25
		d.VariationFactor = 1.0
		d.ErrorRate = 0.001
	}
}

// getRegionMultiplier returns a multiplier based on the device's region
func (d *Device) getRegionMultiplier() float64 {
	switch d.Location.Region {
	case "north":
		return -0.5 // Cooler
	case "south":
		return 0.8 // Warmer
	case "east":
		return 0.2
	case "west":
		return -0.2
	case "central":
		return 0.0
	default:
		return 0.0
	}
}

// GenerateReading generates a realistic sensor reading with time-based variations
func (d *Device) GenerateReading() *SensorReading {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	now := time.Now()

	// Simulate device errors
	if d.random.Float64() < d.ErrorRate {
		d.Status = "error"
		return &SensorReading{
			DeviceID:    d.ID,
			Timestamp:   now,
			Temperature: -999.0, // Error value
			Humidity:    -999.0,
			Pressure:    -999.0,
			Location:    d.Location,
			Status:      "error",
		}
	}

	// Time-based variations (daily cycle)
	hourOfDay := float64(now.Hour())
	dailyCycle := math.Sin((hourOfDay - 6) * math.Pi / 12) // Peak at 6 PM

	// Seasonal variation (simplified)
	dayOfYear := float64(now.YearDay())
	seasonalCycle := math.Sin((dayOfYear - 80) * 2 * math.Pi / 365) // Peak in summer

	// Random variations
	tempVariation := d.random.NormFloat64() * d.VariationFactor
	humidityVariation := d.random.NormFloat64() * d.VariationFactor * 0.8
	pressureVariation := d.random.NormFloat64() * 0.5

	// Calculate realistic values
	temperature := d.BaseTemperature +
		dailyCycle*5 + // ±5°C daily variation
		seasonalCycle*10 + // ±10°C seasonal variation
		tempVariation

	humidity := d.BaseHumidity +
		dailyCycle*(-10) + // Inverse correlation with temperature
		seasonalCycle*5 + // Seasonal humidity changes
		humidityVariation

	pressure := d.BasePressure +
		seasonalCycle*2 + // Seasonal pressure changes
		pressureVariation

	// Ensure realistic bounds
	temperature = math.Max(-50, math.Min(60, temperature))
	humidity = math.Max(0, math.Min(100, humidity))
	pressure = math.Max(950, math.Min(1050, pressure))

	// Update device state
	d.LastReading = now
	d.ReadingCount++
	d.Status = "active"

	// Occasionally simulate maintenance status
	if d.ReadingCount%1000 == 0 && d.random.Float64() < 0.01 {
		d.Status = "maintenance"
	}

	return &SensorReading{
		DeviceID:    d.ID,
		Timestamp:   now,
		Temperature: math.Round(temperature*100) / 100, // 2 decimal places
		Humidity:    math.Round(humidity*100) / 100,
		Pressure:    math.Round(pressure*100) / 100,
		Location:    d.Location,
		Status:      d.Status,
	}
}

// GetStats returns device statistics
func (d *Device) GetStats() map[string]interface{} {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return map[string]interface{}{
		"device_id":     d.ID,
		"type":          d.Type,
		"status":        d.Status,
		"reading_count": d.ReadingCount,
		"last_reading":  d.LastReading,
		"location":      d.Location,
	}
}

// IsHealthy checks if the device is functioning properly
func (d *Device) IsHealthy() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.Status == "active" || d.Status == "maintenance"
}

// SetStatus updates the device status
func (d *Device) SetStatus(status string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.Status = status
}

// GetID returns the device ID (thread-safe)
func (d *Device) GetID() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.ID
}

// GenerateDeviceID generates a unique device ID
func GenerateDeviceID(index int, deviceType DeviceType) string {
	return fmt.Sprintf("%s_%06d", deviceType, index)
}

// GenerateRandomLocation generates a random location within realistic bounds
func GenerateRandomLocation(region string) Location {
	var lat, lng float64

	// Define region bounds (simplified for demo)
	switch region {
	case "north":
		lat = 41.0 + rand.Float64()*2.0 // Istanbul area
		lng = 28.0 + rand.Float64()*4.0
	case "south":
		lat = 36.0 + rand.Float64()*2.0 // Antalya area
		lng = 30.0 + rand.Float64()*4.0
	case "east":
		lat = 38.0 + rand.Float64()*2.0 // Ankara area
		lng = 32.0 + rand.Float64()*4.0
	case "west":
		lat = 38.5 + rand.Float64()*2.0 // Izmir area
		lng = 26.0 + rand.Float64()*4.0
	default: // central
		lat = 39.0 + rand.Float64()*2.0 // Central Turkey
		lng = 35.0 + rand.Float64()*4.0
	}

	return Location{
		Latitude:  math.Round(lat*1000000) / 1000000, // 6 decimal places
		Longitude: math.Round(lng*1000000) / 1000000,
		Region:    region,
	}
}
