package job

import (
	"dsci_runner/types"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// Greet returns a greeting message. The capitalized name makes it an exported function.
func JobQueueFs(r types.JobRequest) {

	project := r.Config.Project
	job_id := r.Config.JobId
	fmt.Printf("start job for project: %s, job_id: %s\n", project, job_id)
	hdir, err := os.UserHomeDir()

	sparky_project_dir := fmt.Sprintf("%s/.sparky/projects/%s", hdir, project)

	err = os.MkdirAll(sparky_project_dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	err = os.MkdirAll(fmt.Sprintf("%s/.triggers", sparky_project_dir), 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	cache_dir := fmt.Sprintf("%s/.sparky/.cache/%s", hdir, job_id)

	err = os.MkdirAll(cache_dir, 0755)

	if err != nil {
		log.Fatalf("Error creating directory:", err)
	}

	var jsonData []byte
	jsonData, err = json.MarshalIndent(r.SparrowdoConfig, "", "  ") // Use MarshalIndent for pretty printing
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	err = os.WriteFile(fmt.Sprintf("%s/config.json", cache_dir), jsonData, 0644)
	if err != nil {
		log.Fatal(err)
	}

	r.Trigger.Cwd = cache_dir

	if r.Config.Sparrowdo.Localhost {
		r.Trigger.Sparrowdo.Docker = ""
		r.Trigger.Sparrowdo.Host = ""
	} else if r.Config.Sparrowdo.Host != "" {
		r.Trigger.Sparrowdo.Docker = ""
		r.Trigger.Sparrowdo.Localhost = false
	} else if r.Config.Sparrowdo.Docker != "" {
		r.Trigger.Sparrowdo.Localhost = false
		r.Trigger.Sparrowdo.Host = ""
	}
	if r.Config.Sparrowdo.Sudo {
		r.Trigger.Sparrowdo.NoSudo = false
	}

	if r.Config.Description != "" {
		r.Trigger.Description = r.Config.Description
	} else {
		r.Trigger.Description = "spawned job"
	}

	var tags []string

	for k, vv := range r.Config.Tags {
		switch v := vv.(type) {
		case string:
			v_safe := strings.ReplaceAll(v, ",", "___comma___")
			v_safe = strings.ReplaceAll(v_safe, "=", "___eq___")
			tags = append(tags, fmt.Sprintf("%s=%s", k, v_safe))
		case int:
			tags = append(tags, fmt.Sprintf("%s=%d", k, v))
		case bool:
			tags = append(tags, fmt.Sprintf("%s", k))
		}
	}

	r.Trigger.Sparrowdo.Tags = strings.Join(tags, ",")
	r.Trigger.Sparrowdo.Conf = "config.json"

	fmt.Printf(
		"job-queue-fs: create trigger file: %s/.triggers/%s\n",
		sparky_project_dir,
		job_id,
	)

	jsonData, err = json.MarshalIndent(r.Trigger, "", "  ") // Use MarshalIndent for pretty printing

	err = os.WriteFile(fmt.Sprintf("%s/.triggers/%s.json", sparky_project_dir, job_id), jsonData, 0644)

	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(fmt.Sprintf("%s/sparrowfile", cache_dir), []byte(r.Sparrowfile), 0644)

	if err != nil {
		log.Fatal(err)
	}

}
