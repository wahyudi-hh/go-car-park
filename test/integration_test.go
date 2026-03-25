package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-car-park/internal/client"
	"go-car-park/internal/config"
	"go-car-park/internal/handler"
	"go-car-park/internal/model"
	svc "go-car-park/internal/service"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTest initializes the entire stack and returns the test server URL and a cleanup function.
func setupTest(t *testing.T, mockJSON string, redisClient *redis.Client) (string, func()) {
	// 1. Mock External API (Government API)
	var mockServer *httptest.Server
	if mockJSON != "" {
		jsonPath := filepath.Join("data", mockJSON)
		mockData, err := os.ReadFile(jsonPath)
		require.NoError(t, err, "Failed to read mock JSON")

		mockServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(mockData)
		}))
	}

	// 2. Set Test Environment
	csvPath, _ := filepath.Abs(filepath.Join("..", "data", "HDBCarparkInformation.csv"))
	os.Setenv("CSV_PATH", csvPath)
	os.Setenv("CACHE_REFRESH_PERIOD_SEC", "10")
	if redisClient != nil {
		os.Setenv("USE_REDIS", "true")
	} else {
		os.Setenv("USE_REDIS", "false")
	}
	if mockServer != nil {
		os.Setenv("LIVE_CARPARK_API_URL", mockServer.URL)
	}

	// 3. Initialize Components
	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveSvc := svc.NewLiveCarParkAvailabilityService(apiClient, redisClient)
	carParkSvc, err := svc.NewCarParkService(cfg, liveSvc)
	require.NoError(t, err)

	// 4. Start App Server
	cpHandler := &handler.CarParkHandler{Service: carParkSvc}
	r := gin.New()
	cpHandler.RegisterRoutes(r)
	appServer := httptest.NewServer(r)

	// 5. Return URL and Cleanup function (The "AfterEach")
	cleanup := func() {
		appServer.Close()
		if mockServer != nil {
			mockServer.Close()
		}
		os.Unsetenv("SUPPORTED_LOT_TYPES") // Clean up specific env tweaks
	}

	return appServer.URL, cleanup
}

// TestEndToEnd runs an integration-style end-to-end test across packages
func TestEndToEnd(t *testing.T) {
	url, teardown := setupTest(t, "availability_success.json", nil)
	defer teardown()

	target := url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&page=0&size=5"
	res, err := http.Get(target)
	assert.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)

	var out model.PagedResponse
	err = json.NewDecoder(res.Body).Decode(&out)
	assert.NoError(t, err)

	// Basic checks: total_elements > 0 and first content item has car_park_no == "ACB"
	assert.Greater(t, out.TotalElements, 0)
	assert.NotEmpty(t, out.Content)
	assert.Equal(t, "ACB", out.Content[0].CarParkNo)
	assert.Equal(t, 2, out.TotalPages)
}

// TestInvalidCoordinates tests the endpoint with invalid coordinates
func TestInvalidCoordinates(t *testing.T) {
	url, teardown := setupTest(t, "", nil) // No mock needed for validation tests
	defer teardown()

	tests := []struct {
		name  string
		query string
	}{
		{"Invalid X", "?user_x=invalid&user_y=31490.4942"},
		{"Invalid Y", "?user_x=30314.7936&user_y=invalid"},
		{"Missing X", "?user_y=31490.4942"},
		{"Missing Y", "?user_x=30314.7936"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := http.Get(url + "/car-parks/nearest" + test.query)
			assert.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, http.StatusBadRequest, res.StatusCode)

			var errorResp map[string]string
			json.NewDecoder(res.Body).Decode(&errorResp)
			assert.Equal(t, "Invalid or missing coordinates (x and y are required)", errorResp["error"])
		})
	}
}

// TestInvalidLotType tests the endpoint with an unsupported lot_type
func TestInvalidLotType(t *testing.T) {
	os.Setenv("SUPPORTED_LOT_TYPES", "C,M")
	url, teardown := setupTest(t, "", nil)
	defer teardown()

	res, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&lot_type=X")
	assert.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

// TestMutexCache tests that availability data is cached using mutex cache
func TestMutexCache(t *testing.T) {
	jsonPath := filepath.Join("data", "availability_success.json")
	mockData, err := os.ReadFile(jsonPath)
	require.NoError(t, err, "Failed to read mock JSON")

	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++ // Increment every time the "Government" API is hit
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockData)
	}))
	defer mockServer.Close()

	os.Setenv("LIVE_CARPARK_API_URL", mockServer.URL)
	os.Setenv("SUPPORTED_LOT_TYPES", "C,M") // Only support C and M for this test

	url, teardown := setupTest(t, "", nil)
	defer teardown()

	// First call
	// additional case to cover (page > total_pages)
	res1, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&lot_type=C&page=200")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res1.StatusCode)
	assert.Equal(t, 1, callCount, "First call should hit the external API")
	var out model.PagedResponse
	err = json.NewDecoder(res1.Body).Decode(&out)
	assert.Equal(t, 0, len(out.Content))
	res1.Body.Close()

	// Second call, should use cache
	res2, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&lot_type=C")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res2.StatusCode)
	assert.Equal(t, 1, callCount, "Second call should return from Mutex cache, NOT the API")
	err = json.NewDecoder(res2.Body).Decode(&out)
	assert.True(t, len(out.Content) > 0)
	res2.Body.Close()
}

// TestRedisCache tests that availability data is cached using Redis cache
func TestRedisCache(t *testing.T) {
	// Start a miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	callCount := 0
	jsonPath := filepath.Join("data", "availability_success.json")
	mockData, err := os.ReadFile(jsonPath)
	require.NoError(t, err, "Failed to read mock JSON")
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockData)
	}))
	defer mock.Close()

	os.Setenv("REDIS_HOST", mr.Host())
	os.Setenv("REDIS_PORT", mr.Port())
	os.Setenv("LIVE_CARPARK_API_URL", mock.URL)

	url, teardown := setupTest(t, "", redisClient)
	defer teardown()

	cacheKey := "live_carpark_availability"

	// First call
	res1, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&lot_type=C")
	assert.NoError(t, err)
	res1.Body.Close()
	assert.Equal(t, http.StatusOK, res1.StatusCode)
	assert.Equal(t, 1, callCount, "First call should hit the external API")
	assert.True(t, mr.Exists(cacheKey), "Data should be stored in Redis")

	ttl := mr.TTL(cacheKey)
	assert.Greater(t, ttl.Seconds(), 0.0)

	// Second call, should use cache
	res2, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&page=1&size=5")
	assert.NoError(t, err)
	res2.Body.Close()
	assert.Equal(t, http.StatusOK, res2.StatusCode)
	assert.Equal(t, 1, callCount, "Second call should return from Redis cache, NOT the API")
}

// TestInvalidCSVPath tests error handling when CSV file path is invalid
func TestInvalidCSVPath(t *testing.T) {
	os.Setenv("CSV_PATH", "/nonexistent/path.csv")
	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveSvc := svc.NewLiveCarParkAvailabilityService(apiClient, nil)
	_, err := svc.NewCarParkService(cfg, liveSvc)
	assert.Error(t, err)
}

// TestInvalidCSVContent tests error handling when CSV content is invalid
func TestInvalidCSVContent(t *testing.T) {
	// Create a temporary file with invalid CSV content
	tempFile, err := os.CreateTemp("", "invalid*.csv")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())

	// Write invalid CSV: x_coord is not a number
	_, err = tempFile.WriteString("car_park_no,address,x_coord,y_coord\nTEST,TEST,invalid_float,31490.4942")
	require.NoError(t, err)
	tempFile.Close()

	os.Setenv("CSV_PATH", tempFile.Name())
	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveSvc := svc.NewLiveCarParkAvailabilityService(apiClient, nil)
	_, err = svc.NewCarParkService(cfg, liveSvc)
	assert.Error(t, err)
}

// TestEndToEnd runs an integration-style end-to-end test across packages
func TestInvalidAvailabilityJsonResponse(t *testing.T) {
	url, teardown := setupTest(t, "availability_invalid_json.json", nil)
	defer teardown()

	target := url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&page=0&size=5"
	res, err := http.Get(target)
	assert.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)

	var out model.PagedResponse
	err = json.NewDecoder(res.Body).Decode(&out)
	assert.NoError(t, err)
	assert.Equal(t, 0, out.TotalElements)
	assert.Empty(t, out.Content)
	assert.Equal(t, 0, out.TotalPages)
}

// TestExternalAPI500 tests the behavior when the external API returns a 500 error
func TestExternalAPI500(t *testing.T) {
	// Start a miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	os.Setenv("LIVE_CARPARK_API_URL", mockServer.URL)

	url, teardown := setupTest(t, "", redisClient)
	defer teardown()

	res, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&&page=1")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var out model.PagedResponse
	err = json.NewDecoder(res.Body).Decode(&out)
	assert.Equal(t, 0, out.TotalElements)
	assert.Empty(t, out.Content)
	assert.Equal(t, 0, out.TotalPages)
	res.Body.Close()
}

// TestExternalAPITimeout tests the behavior when the external API times out
func TestExternalAPITimeout(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Sleep for 2 seconds
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	os.Setenv("LIVE_CARPARK_API_URL", mockServer.URL)
	os.Setenv("API_TIMEOUT_SEC", "1") // Set a short timeout for testing

	url, teardown := setupTest(t, "", nil)
	defer teardown()

	res, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&&page=1")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var out model.PagedResponse
	err = json.NewDecoder(res.Body).Decode(&out)
	assert.Equal(t, 0, out.TotalElements)
	assert.Empty(t, out.Content)
	assert.Equal(t, 0, out.TotalPages)
	res.Body.Close()
}
