package model

type CarPark struct {
	CarParkNo string  `csv:"car_park_no" json:"car_park_no"`
	Address   string  `csv:"address" json:"address"`
	XCoord    float64 `csv:"x_coord" json:"x_coord"`
	YCoord    float64 `csv:"y_coord" json:"y_coord"`
}

type CarParkInfo struct {
	TotalLots     int    `json:"totalLots"`
	LotType       string `json:"lotType"`
	LotsAvailable int    `json:"lotsAvailable"`
}

type CarParkResponse struct {
	CarParkInfo    []CarParkInfo `json:"car_park_info"`
	CarParkNo      string        `json:"car_park_no"`
	Distance       float64       `json:"distance_meters"`
	UpdateDateTime string        `json:"update_datetime,omitempty"`
	XCoord         float64       `json:"x_coord"`
	YCoord         float64       `json:"y_coord"`
}

type PagedResponse struct {
	Content       []CarParkResponse `json:"content"`
	CurrentPage   int               `json:"current_page"`
	TotalElements int               `json:"total_elements"`
	TotalPages    int               `json:"total_pages"`
}
