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
)

// TestEndToEnd runs an integration-style end-to-end test across packages
func TestEndToEnd(t *testing.T) {
	// 1) Mock external availability API
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := model.AvailabilityResponse{
			Items: []struct {
				Timestamp   string                  `json:"timestamp"`
				CarParkData []model.LiveCarParkData `json:"carpark_data"`
			}{
				{
					Timestamp: "2026-03-24T00:00:00+00:00",
					CarParkData: []model.LiveCarParkData{
						{
							CarParkNumber:  "ACB",
							UpdateDateTime: "2026-03-24T00:00:00",
							CarParkInfo: []model.LiveCarParkInfo{
								{TotalLots: "100", LotType: "C", LotsAvailable: "50"},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mock.Close()

	// 2) Set environment for test (prefer fixture in integration/fixtures)
	csvPath := filepath.Join("fixtures", "HDBCarparkInformation.csv")
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		csvPath = filepath.Join("..", "data", "HDBCarparkInformation.csv")
	}
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
	carParkSvc, err := svc.NewCarParkService(cfg, liveSvc)
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
	url := srv.URL + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&page=1&size=10"
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("failed decode response: %v", err)
	}

	// Basic checks: total_elements > 0 and first content item has car_park_no == "ACB"
	total, ok := out["total_elements"].(float64)
	if !ok || total <= 0 {
		t.Fatalf("expected total_elements > 0, got: %v", out["total_elements"])
	}

	content, ok := out["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty content, got: %v", out["content"])
	}
	first, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content item type: %T", content[0])
	}
	cpNo, _ := first["car_park_no"].(string)
	if cpNo != "ACB" {
		t.Fatalf("expected first car park to be ACB, got: %v", cpNo)
	}
}

// helper to setup test server and mock API; returns server URL and cleanup
func setupServerWithMock(t *testing.T, supportedLotTypes string) (srvURL string, cleanup func()) {
	t.Helper()

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := model.AvailabilityResponse{
			Items: []struct {
				Timestamp   string                  `json:"timestamp"`
				CarParkData []model.LiveCarParkData `json:"carpark_data"`
			}{
				{
					Timestamp: "2026-03-24T00:00:00+00:00",
					CarParkData: []model.LiveCarParkData{
						{
							CarParkNumber:  "ACB",
							UpdateDateTime: "2026-03-24T00:00:00",
							CarParkInfo: []model.LiveCarParkInfo{
								{TotalLots: "100", LotType: "C", LotsAvailable: "50"},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	// CSV path: prefer integration fixture
	csvPath := filepath.Join("fixtures", "HDBCarparkInformation.csv")
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		csvPath = filepath.Join("..", "data", "HDBCarparkInformation.csv")
	}
	if abs, err := filepath.Abs(csvPath); err == nil {
		csvPath = abs
	}

	_ = os.Setenv("CSV_PATH", csvPath)
	_ = os.Setenv("LIVE_CARPARK_API_URL", mock.URL)
	_ = os.Setenv("USE_REDIS", "false")
	_ = os.Setenv("API_TIMEOUT_SEC", "5")
	_ = os.Setenv("CACHE_REFRESH_PERIOD_SEC", "1")
	if supportedLotTypes != "" {
		_ = os.Setenv("SUPPORTED_LOT_TYPES", supportedLotTypes)
	} else {
		_ = os.Unsetenv("SUPPORTED_LOT_TYPES")
	}

	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveSvc := svc.NewLiveCarParkAvailabilityService(apiClient, nil)
	carParkSvc, err := svc.NewCarParkService(cfg, liveSvc)
	if err != nil {
		mock.Close()
		t.Fatalf("failed to init car park service: %v", err)
	}

	cpHandler := &handler.CarParkHandler{Service: carParkSvc}
	r := gin.New()
	cpHandler.RegisterRoutes(r)
	srv := httptest.NewServer(r)

	cleanup = func() {
		srv.Close()
		mock.Close()
	}
	return srv.URL, cleanup
}

func TestInvalidCoordinates(t *testing.T) {
	url, cleanup := setupServerWithMock(t, "")
	defer cleanup()

	res, err := http.Get(url + "/car-parks/nearest?user_x=not-a-number&user_y=31490.4942&page=1&size=10")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid coordinates, got %d", res.StatusCode)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("failed decode response: %v", err)
	}
	if out["error"] != "Invalid or missing coordinates (x and y are required)" {
		t.Fatalf("unexpected error message: %v", out["error"])
	}
}

func TestUnsupportedLotType(t *testing.T) {
	// Restrict supported lot types to only "C" so "Z" is unsupported
	url, cleanup := setupServerWithMock(t, "C")
	defer cleanup()

	res, err := http.Get(url + "/car-parks/nearest?user_x=30314.7936&user_y=31490.4942&lot_type=Z&page=1&size=10")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported lot type, got %d", res.StatusCode)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("failed decode response: %v", err)
	}
	if out["error"] != "Unsupported lot type" {
		t.Fatalf("unexpected error message: %v", out["error"])
	}
}
