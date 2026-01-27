package types

type Config struct {
	Project string `json:"project"`
	JobId   string `json:"job-id"`
}

type JobRequest struct {
	Config          Config      `json:"config"`
	Trigger         interface{} `json:"trigger"`
	Sparrowfile     string      `json:"sparrowfile"`
	SparrowdoConfig interface{} `json:"sparrowdo-config"`
}
