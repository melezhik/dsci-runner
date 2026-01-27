package types

type Config struct {
	Project string `json:"project"`
	JobId   string `json:"job-id"`
}

type Sparrowdo struct {
	Localhost Bool `json:"localhost"`
	NoSudo Bool `json:"no_sudo"`
	Conf string `json:"conf"`
	Host string `json:"host"`
	Docker string `json:"docker"`
	Image string `json:"images"`
	SshUser string `json:"ssh_user"`
	Tags string `json:"tags"`
}

type Trigger struct {
	Cwd string `json:"project"`
	Description string `json:"description"`
	Sparrowdo Sparrowdo`json:"sparrowdo"`
}

type JobRequest struct {
	Config          Config      `json:"config"`
	Trigger         interface{} `json:"trigger"`
	Sparrowfile     string      `json:"sparrowfile"`
	SparrowdoConfig interface{} `json:"sparrowdo-config"`
}
