package integration_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-car-park/internal/client"
	"go-car-park/internal/config"
	"go-car-park/internal/handler"
	"go-car-park/internal/model"
	svc "go-car-park/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestEndToEnd runs an integration-style end-to-end test across packages
func TestEndToEnd(t *testing.T) {
	// 1) Mock external availability API
	jsonPath := filepath.Join("data", "availability_success.json")

	mockData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("Failed to read mock JSON file: %v", err)
	}
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockData)
	}))
	defer mock.Close()

	// 2) Set environment for test (use mutex cache, CSV file in repo root)
	csvPath := filepath.Join("..", "data", "HDBCarparkInformation.csv")
	if abs, err := filepath.Abs(csvPath); err == nil {
		csvPath = abs
	}
	_ = os.Setenv("CSV_PATH", csvPath)
	_ = os.Setenv("LIVE_CARPARK_API_URL", mock.URL)
	_ = os.Setenv("USE_REDIS", "false")
	_ = os.Setenv("API_TIMEOUT_SEC", "5")
	_ = os.Setenv("CACHE_REFRESH_PERIOD_SEC", "1")

	// 3) Initialize components as application would
	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveSvc := svc.NewLiveCarParkAvailabilityService(apiClient, nil)
	carParkSvc, err := svc.NewCarParkService(cfg.CSVPath, liveSvc)
	if err != nil {
		t.Fatalf("failed to init car park service: %v", err)
	}

	// 4) Wire handler + router and run an httptest server
	cpHandler := &handler.CarParkHandler{Service: carParkSvc}
	r := gin.New()
	cpHandler.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	// 5) Call the /car-parks/nearest endpoint with coordinates of ACB (from CSV)
	url := srv.URL + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&page=1&size=5"
	res, err := http.Get(url)
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
