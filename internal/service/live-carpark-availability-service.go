package service

import (
	"log"
	"sync"
	"time"

	"go-car-park/internal/client"
	"go-car-park/internal/config"
	"go-car-park/internal/model"
)

type LiveCarParkAvailabilityService struct {
	apiClient *client.AvailabilityClient

	// Cache for live data
	liveCarparkCache model.AvailabilityResponse
	lastFetchTime    time.Time
	cacheMutex       sync.RWMutex // To make it Thread-Safe (like a ConcurrentHashMap)
}

func NewLiveCarParkAvailabilityService(apiClient *client.AvailabilityClient) *LiveCarParkAvailabilityService {
	return &LiveCarParkAvailabilityService{
		apiClient: apiClient,
	}
}

func (s *LiveCarParkAvailabilityService) getLatestAvailability() model.AvailabilityResponse {
	log.Println("Fetching latest availability data...")
	cfg := config.LoadConfig()
	// 1. Check if cache is still fresh (e.g., less than 1 minute old)
	s.cacheMutex.RLock()
	if time.Since(s.lastFetchTime) < cfg.CacheRefreshPeriod && s.liveCarparkCache.Items != nil {
		log.Println("Using cached availability data")
		defer s.cacheMutex.RUnlock()
		return s.liveCarparkCache
	}
	s.cacheMutex.RUnlock()

	// 2. Cache is stale, let's update it
	log.Println("Cache is stale, fetching new data from external API")
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// 3. Perform the actual "Feign" call
	liveData, err := s.apiClient.FetchAvailability()
	if err != nil {
		// Log the error and return the old cache if available
		log.Printf("Error fetching live availability: %v", err)
		if s.liveCarparkCache.Items != nil {
			return s.liveCarparkCache
		}
		return model.AvailabilityResponse{} // Return empty if no cache
	}

	s.liveCarparkCache = *liveData
	s.lastFetchTime = time.Now()
	return s.liveCarparkCache
}
