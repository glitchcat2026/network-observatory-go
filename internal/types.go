package internal

type PingResult struct {
	Success    bool
	MinRTT     float64
	MaxRTT     float64
	AvgRTT     float64
	PacketLoss float64
	Datetime   string `json:"datetime"`
}

type Report struct {
	Probe_name string
	Token      string
	IPVersion  string `json:"ip_version"`
	Results    map[string]PingResult
}
