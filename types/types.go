package types

type Config struct {
	Project     string                 `json:"project"`
	Description string                 `json:"description"`
	JobId       string                 `json:"job-id"`
	Sparrowdo   Sparrowdo              `json:"sparrowdo"`
	Tags        map[string]interface{} `json:"tags"`
}

type Sparrowdo struct {
	Localhost bool   `json:"localhost"`
	NoSudo    bool   `json:"no_sudo"`
	Sudo      bool   `json:"no_sudo"`
	Conf      string `json:"conf"`
	Host      string `json:"host"`
	Docker    string `json:"docker"`
	Image     string `json:"images"`
	SshUser   string `json:"ssh_user"`
	Tags      string `json:"tags"`
}

type Trigger struct {
	Cwd         string    `json:"cwd"`
	Description string    `json:"description"`
	Sparrowdo   Sparrowdo `json:"sparrowdo"`
}

type JobRequest struct {
	Config          Config      `json:"config"`
	Trigger         Trigger     `json:"trigger"`
	Sparrowfile     string      `json:"sparrowfile"`
	SparrowdoConfig interface{} `json:"sparrowdo-config"`
}
