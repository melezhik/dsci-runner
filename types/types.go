package types

type AppConfig struct {
	DsciFeedbackUrl        			string
	DsciAgentSkipBootstrap 			bool
	DsciAgentImage         			string
	DsciAllowLocalhostModeRepos 	[]string
 	DsciContainerRuntime            string
 	GitPathToHttpBackend            string
	GitServerAddress			    string	
}

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
	Bootstrap bool   `json:"bootstrap"`
	Sudo      bool   `json:"sudo"`
	Conf      string `json:"conf"`
	Host      string `json:"host"`
	Docker    string `json:"docker"`
	Image     string `json:"image"`
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

type JobStashConfig struct {
	Project string `json:"project"`
	JobId   string `json:"job-id"`
}
type JobStash struct {
	Config JobStashConfig `json:"config"`
	Data   map[string]interface{}
}

type StashRequest struct {
	Config Config      `json:"config"`
	Data   interface{} `json:"data"`
}

type JobBuild struct {
	ID          int
	Project     string
	JobId       string
	Description string
	Dt          string
	State       int
}
