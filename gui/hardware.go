package gui

// SystemCapacity describes the system's RAM and GPU resources.
type SystemCapacity struct {
	TotalRAMGB   float64 `json:"ram_gb"`
	HasNVIDIA    bool    `json:"has_gpu"`
	NVIDIAVRAMGB float64 `json:"vram_gb"`
}
