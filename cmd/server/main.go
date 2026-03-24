package main

import (
	"log"

	"go-car-park/internal/client"
	"go-car-park/internal/config"
	"go-car-park/internal/handler"
	"go-car-park/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Configuration
	cfg := config.LoadConfig()
	apiClient := client.NewAvailabilityClient(cfg)
	liveCarParkService := service.NewLiveCarParkAvailabilityService(apiClient)

	// 2. Initialize Service (The @Service / @PostConstruct equivalent)
	carParkService, err := service.NewCarParkService(cfg.CSVPath, liveCarParkService)
	if err != nil {
		log.Fatalf("Could not initialize service: %v", err)
	}

	// 3. Initialize Handler (The @RestController equivalent)
	carParkHandler := &handler.CarParkHandler{
		Service: carParkService,
	}

	// 4. Setup Router
	r := gin.Default()

	// 5. Define Routes
	// Just call the registration function
	carParkHandler.RegisterRoutes(r)

	// If you add a UserHandler later:
	// userHandler.RegisterRoutes(r)

	// 6. Start Server
	log.Println("Server starting on :8081...")
	r.Run(":8081")
}
