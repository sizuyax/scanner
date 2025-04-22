package product_state

type ScanHistory struct {
	Ean      string `json:"ean"`
	Quantity uint   `json:"quantity"`
}

var ScanHistArr []ScanHistory
