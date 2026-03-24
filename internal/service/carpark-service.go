package service

import (
	"log"
	"math"
	"os"
	"sort"
	"strconv"

	"go-car-park/internal/model"

	"github.com/gocarina/gocsv"
)

type CarParkService struct {
	// Our in-memory cache (The 2,270 records)
	cache                   map[string]model.CarPark
	liveCarpackAvailability *LiveCarParkAvailabilityService
}

// NewCarParkService acts like your @PostConstruct / Bean Initialization
func NewCarParkService(filePath string, liveCarParkService *LiveCarParkAvailabilityService) (*CarParkService, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []model.CarPark
	// Unmarshal CSV directly into our slice of structs
	if err := gocsv.UnmarshalFile(file, &data); err != nil {
		log.Printf("Error parsing CSV: %v", err)
		return nil, err
	}

	cache := make(map[string]model.CarPark)
	for _, cp := range data {
		cache[cp.CarParkNo] = cp
	}

	return &CarParkService{
		cache:                   cache,
		liveCarpackAvailability: liveCarParkService,
	}, nil
}

// GetPagedNearest handles the Euclidean math and pagination
func (s *CarParkService) GetPagedNearest(userX, userY float64, page, size int, lotType string) model.PagedResponse {
	// 1. Fetch Live Data (check cache first if empty then Feign Call)
	availabilityData := s.liveCarpackAvailability.getLatestAvailability()

	// 2. Filter and Calculate distances
	var responses []model.CarParkResponse
	for _, lcp := range availabilityData.Items[0].CarParkData {
		carPark, foundCP := s.cache[lcp.CarParkNumber]
		if !foundCP {
			continue // Skip if no static data for this car park
		}
		var infos []model.CarParkInfo
		for _, info := range lcp.CarParkInfo {
			infos = append(infos, model.CarParkInfo{
				TotalLots:     atoi(info.TotalLots),
				LotType:       info.LotType,
				LotsAvailable: atoi(info.LotsAvailable),
			})
		}
		dist := s.calculateEuclidean(userX, userY, carPark.XCoord, carPark.YCoord)
		responses = append(responses, model.CarParkResponse{
			CarParkInfo:    infos,
			CarParkNo:      carPark.CarParkNo,
			Distance:       dist,
			UpdateDateTime: lcp.UpdateDateTime,
			XCoord:         carPark.XCoord,
			YCoord:         carPark.YCoord,
		})
	}

	// 2. Sort by distance (Ascending)
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].Distance < responses[j].Distance
	})

	// 3. Pagination Logic (The Stream .skip().limit() equivalent)
	if page < 1 {
		page = 1
	}
	start := (page - 1) * size
	if start >= len(responses) {
		var pagedResponse model.PagedResponse
		pagedResponse.CurrentPage = page
		pagedResponse.TotalElements = len(responses)
		pagedResponse.TotalPages = (len(responses) + size - 1) / size
		return pagedResponse
	}

	end := start + size
	if end > len(responses) {
		end = len(responses)
	}

	// return responses[start:end]
	var pagedResponse model.PagedResponse
	pagedResponse.Content = responses[start:end]
	pagedResponse.CurrentPage = page
	pagedResponse.TotalElements = len(responses)
	pagedResponse.TotalPages = (len(responses) + size - 1) / size

	return pagedResponse
}

func atoi(s string) int {
	num, err := strconv.Atoi(s)
	if err != nil {
		return 0 // Default to 0 if conversion fails
	}
	return int(num)
}

// Euclidean distance calculation (Meters for SVY21)
func (s *CarParkService) calculateEuclidean(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}
