package model

// LiveAvailability represents the real-time data from an external API
type LiveCarParkInfo struct {
	TotalLots     string `json:"total_lots"`
	LotType       string `json:"lot_type"`
	LotsAvailable string `json:"lots_available"`
}

type LiveCarParkData struct {
	CarParkInfo    []LiveCarParkInfo `json:"carpark_info"`
	CarParkNumber  string            `json:"carpark_number"`
	UpdateDateTime string            `json:"update_datetime"`
}

// APIResponse matches the typical wrapper of external APIs
type AvailabilityResponse struct {
	Items []struct {
		Timestamp   string            `json:"timestamp"`
		CarParkData []LiveCarParkData `json:"carpark_data"`
	} `json:"items"`
}
