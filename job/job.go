package job

import (
	"dsci_runner/types"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
}
