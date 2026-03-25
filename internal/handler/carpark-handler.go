package handler

import (
	"net/http"
	"strconv"

	"go-car-park/internal/service"

	"github.com/gin-gonic/gin"
)

type CarParkHandler struct {
	// Dependency Injection: The handler needs the service
	Service *service.CarParkService
}

// GetNearest handles GET /car-parks/nearest
func (h *CarParkHandler) GetNearest(c *gin.Context) {
	// 1. Parse Query Parameters (with default values)
	// Example: /car-parks/nearest?x=30456.12&y=31234.56&page=0&size=10

	xStr := c.Query("user_x")
	yStr := c.Query("user_y")
	pageStr := c.DefaultQuery("page", "1")
	sizeStr := c.DefaultQuery("size", "10")
	lotType := c.Query("lot_type")

	// 2. Convert Strings to Numeric types (Go requires explicit conversion)
	x, errX := strconv.ParseFloat(xStr, 64)
	y, errY := strconv.ParseFloat(yStr, 64)

	// Basic Validation: If X or Y are missing/invalid
	if errX != nil || errY != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid or missing coordinates (x and y are required)",
		})
		return
	}

	if lotType != "" && !h.Service.IsLotTypeSupported(lotType) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported lot type",
		})
		return
	}

	page, _ := strconv.Atoi(pageStr)
	size, _ := strconv.Atoi(sizeStr)

	// 3. Call the Service Logic
	results := h.Service.GetPagedNearest(x, y, page, size, lotType)

	// 4. Return the results as JSON
	// Gin automatically serializes your structs based on the `json` tags
	c.JSON(http.StatusOK, results)
}

func (h *CarParkHandler) RegisterRoutes(r *gin.Engine) {
	// Group all routes starting with /car-parks
	cp := r.Group("/car-parks")
	{
		cp.GET("/nearest", h.GetNearest)
		// You can easily add more here later:
		// cp.GET("/:id", h.GetByID)
		// cp.POST("/", h.Create)
	}
}
