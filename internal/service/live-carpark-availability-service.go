package service

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"go-car-park/internal/client"
	"go-car-park/internal/config"
	"go-car-park/internal/model"

	"github.com/redis/go-redis/v9"
)

type LiveCarParkAvailabilityService struct {
	apiClient *client.AvailabilityClient

	// Cache for live data
	liveCarparkCache model.AvailabilityResponse
	lastFetchTime    time.Time
	cacheMutex       sync.RWMutex  // To make it Thread-Safe (like a ConcurrentHashMap)
	rdb              *redis.Client // Redis client for distributed caching, replaces mutex usage
}

func NewLiveCarParkAvailabilityService(apiClient *client.AvailabilityClient, rdb *redis.Client) *LiveCarParkAvailabilityService {
	return &LiveCarParkAvailabilityService{
		apiClient: apiClient,
		rdb:       rdb,
	}
}

func (s *LiveCarParkAvailabilityService) getLatestAvailability() model.AvailabilityResponse {
	log.Println("Fetching latest availability data...")
	cfg := config.LoadConfig()
	if cfg.UseRedis {
		return s.fetchRedisCache(cfg)
	}
	return s.fetchMutexCache(cfg)
}

func (s *LiveCarParkAvailabilityService) fetchMutexCache(cfg *config.Config) model.AvailabilityResponse {
	// 1. Check if cache is still fresh (e.g., less than 1 minute old)
	s.cacheMutex.RLock()
	if time.Since(s.lastFetchTime) < cfg.CacheRefreshPeriod && s.liveCarparkCache.Items != nil {
		log.Println("Using cached availability data")
		defer s.cacheMutex.RUnlock()
		return s.liveCarparkCache
	}
	s.cacheMutex.RUnlock()

	// 2. Cache is stale, update it
	log.Println("Cache is stale, fetching new data from external API")
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Perform the actual "Feign" call
	liveData := s.fetchLiveData()

	s.liveCarparkCache = liveData
	s.lastFetchTime = time.Now()

	return s.liveCarparkCache
}

func (s *LiveCarParkAvailabilityService) fetchRedisCache(cfg *config.Config) model.AvailabilityResponse {
	// Redis-based cache check
	// 1. Try to get from Redis
	ctx := context.Background()
	cacheKey := "live_carpark_availability"
	lockKey := "live_carpark_availability_lock"
	cachedData, err := s.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		log.Println("Using cached availability data from Redis")
		return UnmarshalFile(cachedData)
	} else if err != redis.Nil {
		log.Printf("Failed accessing Redis cache Fallback to fetch External API. Error: %v", err)
		return s.fetchLiveData()
	}

	// 2.Perform the actual "Feign" call
	liveData := s.fetchLiveData()

	// 3. Save to Redis
	// To prevent cache stampede, use Redis SET with NX option
	// to prevent multiple processes from updating the cache simultaneously (acts like a distributed lock)
	defer s.rdb.Del(ctx, lockKey) // Cleanup the lock when finished
	err = s.rdb.SetArgs(ctx, lockKey, "live carpark availability locked", redis.SetArgs{
		Mode: "NX", // Only set if not exists
		TTL:  10 * time.Second,
	}).Err()
	if err != nil {
		// Lock failed! Someone else is updating.
		// Wait and "Poll" Redis every 100ms until the data appears
		log.Println("Another process is updating the cache, waiting for it to finish...")
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 50; i++ { // Try for up to 5 seconds
			<-ticker.C
			val, _ := s.rdb.Get(ctx, cacheKey).Result()
			if val != "" {
				return UnmarshalFile(val)
			}
		}
		return model.AvailabilityResponse{} // Timeout
	}

	// Save actual data to Redis for the configured period
	jsonData, _ := json.Marshal(liveData)
	s.rdb.Set(ctx, cacheKey, jsonData, cfg.CacheRefreshPeriod)

	return liveData
}

func (s *LiveCarParkAvailabilityService) fetchLiveData() model.AvailabilityResponse {
	log.Println("Fetching live data from external API...")
	liveData, err := s.apiClient.FetchAvailability()
	if err != nil {
		// Log the error and return the old cache if available
		log.Printf("Error fetching live availability: %v", err)
		if s.liveCarparkCache.Items != nil {
			return s.liveCarparkCache
		}
		return model.AvailabilityResponse{} // Return empty if no cache
	}

	// Filter only available carparks
	liveCarParkDatas := liveData.Items[0].CarParkData
	n := 0
	for _, data := range liveCarParkDatas {
		availableCount := 0
		for _, info := range data.CarParkInfo {
			availableCount += atoi(info.LotsAvailable)
		}
		if availableCount > 0 {
			liveCarParkDatas[n] = data
			n++
		}
	}
	liveData.Items[0].CarParkData = liveCarParkDatas[:n]
	return *liveData
}

func UnmarshalFile(data string) (result model.AvailabilityResponse) {
	json.Unmarshal([]byte(data), &result)
	return result
}
