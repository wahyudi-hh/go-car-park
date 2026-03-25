package service

import (
	"log"
	"math"
	"os"
	"slices"
	"sort"
	"strconv"

	"go-car-park/internal/config"
	"go-car-park/internal/model"

	"github.com/gocarina/gocsv"
)

type CarParkService struct {
	// Our in-memory cache (The 2,270 records)
	carParkCache            map[string]model.CarPark
	liveCarpackAvailability *LiveCarParkAvailabilityService
	supportedLotTypes       []string
}

// NewCarParkService acts like your @PostConstruct / Bean Initialization
func NewCarParkService(cfg *config.Config, liveCarParkService *LiveCarParkAvailabilityService) (*CarParkService, error) {
	file, err := os.Open(cfg.CSVPath)
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

	carParkMap := make(map[string]model.CarPark)
	for _, cp := range data {
		carParkMap[cp.CarParkNo] = cp
	}

	return &CarParkService{
		carParkCache:            carParkMap,
		liveCarpackAvailability: liveCarParkService,
		supportedLotTypes:       cfg.SupportedLotTypes,
	}, nil
}

// GetPagedNearest handles the Euclidean math and pagination
func (s *CarParkService) GetPagedNearest(userX, userY float64, page, size int, lotType string) model.PagedResponse {
	// 1. Fetch Live Data (check cache first if empty then Feign Call)
	availabilityData := s.liveCarpackAvailability.getLatestAvailability()
	if availabilityData.Items == nil {
		return fallbackResponse(nil, page, size)
	}
	carParkData := availabilityData.Items[0].CarParkData

	// 2. Filter and Calculate distances
	if lotType != "" {
		carParkData = filterByLotType(carParkData, lotType)
	}
	var responses []model.CarParkResponse
	for _, lcp := range carParkData {
		carPark, foundCP := s.carParkCache[lcp.CarParkNumber]
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
		dist := calculateEuclidean(userX, userY, carPark.XCoord, carPark.YCoord)
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
		return fallbackResponse(responses, page, size)
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

func fallbackResponse(responses []model.CarParkResponse, page int, size int) model.PagedResponse {
	var pagedResponse model.PagedResponse
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
func calculateEuclidean(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

func (s *CarParkService) IsLotTypeSupported(lotType string) bool {
	if s.supportedLotTypes == nil {
		return true // If no filter is set, consider all lot types as supported
	}
	return slices.Contains(s.supportedLotTypes, lotType)
}

func filterByLotType(src []model.LiveCarParkData, lotType string) []model.LiveCarParkData {
	n := 0
	for _, item := range src {
		for _, info := range item.CarParkInfo {
			if (info.LotType == lotType) && atoi(info.LotsAvailable) > 0 {
				src[n] = item
				n++
				break // No need to check other infos for this car park
			}
		}
	}
	return src[:n]
}
