package main

import (
	//"errors"
	"dsci_runner/job"
	"dsci_runner/types"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLogger()) // use the RequestLogger middleware with slog logger
	e.Use(middleware.Recover())       // recover panics as errors for proper error handling

	// Routes
	e.GET("/", hello)
	e.POST("/queue", queue_job)
	e.POST("/stash", put_job_stash)
	e.GET("/stash/:project/:key", get_job_stash)
	e.POST("/forgejo_hook", forgejo_hook)
	e.GET("/status/:project/:key", status)
	e.GET("/report/raw/:project/:key", report)
	e.GET("/trigger/:project/:key", trigger)

	// Start server
	if err := e.Start(":8080"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}

// Handlers

func hello(c *echo.Context) error {
	return c.String(http.StatusOK, "Hello from DSCI runner!")
}

func queue_job(c *echo.Context) error {

	var r types.JobRequest

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	defer c.Request().Body.Close()

	log.Printf("Request Body: %s\n", string(bodyBytes))

	err = json.Unmarshal(bodyBytes, &r)

	if err != nil {
		log.Printf("queue_job: error unmarshaling JSON: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	}

	job.JobQueueFs(r)

	return c.String(http.StatusOK, "job queued")

}

func put_job_stash(c *echo.Context) error {

	var r types.StashRequest

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	defer c.Request().Body.Close()

	log.Printf("Request Body: %s\n", string(bodyBytes))

	err = json.Unmarshal(bodyBytes, &r)

	if err != nil {
		log.Printf("put_job_stash: error unmarshaling JSON: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	}

	job.PutJobStash(r.Config.Project, r.Config.JobId, r.Data)

	return c.String(http.StatusOK, "file created")

}

func get_job_stash(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	stash_json := []byte(job.GetJobStash(project, job_id))

	var p interface{}

	err := json.Unmarshal(stash_json, &p)

	if err != nil {
		log.Fatalf("get_job_stash: put_job_stash json.Unmarshel error: %v", err)
	}

	return c.JSON(http.StatusOK, p)

}

func forgejo_hook(c *echo.Context) error {

	var r types.ForgejoHook

	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	defer c.Request().Body.Close()

	log.Printf("Request Body: %s\n", string(bodyBytes))

	err = json.Unmarshal(bodyBytes, &r)

	if err != nil {
		log.Printf("forgejo_hook: error unmarshaling JSON: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	}

	// job.PutJobStash(r.Config.Project,r.Config.JobId,r.Data)

	var q types.JobRequest
	now := time.Now()
	q.Config.Project = "dsci"
	q.Config.JobId = strconv.FormatInt(now.Unix(), 10)
	q.Config.Description = fmt.Sprintf("%s | %s", r.Sha, r.HeadCommit.Message)
	q.Trigger.Sparrowdo.Tags = fmt.Sprintf(
		"ref=%s,repo_full_name=%s,sha=%s,scm=%s,message=%s",
		r.Ref,
		r.Repository.FullName,
		r.Sha,
		r.Repository.CloneUrl,
		r.HeadCommit.Message,
	)

	q.Trigger.Sparrowdo.NoSudo = true

	q.Trigger.Sparrowdo.Localhost = true

	dat := job.GetSparkyScenarioFile("dsci", "sparrowfile")

	q.Sparrowfile = dat

	// jsonData, err := json.MarshalIndent(q, "", "  ")

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(string(jsonData))

	// log.Printf("dsci job: %s",jsonData)

	job.JobQueueFs(q)

	return c.String(http.StatusOK, fmt.Sprintf("job quedued: %s", q.Config.JobId))

}

func status(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	state := job.JobState(project, job_id)

	return c.String(http.StatusOK, state)

}

func report(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	data := job.Report(project, job_id)

	return c.String(http.StatusOK, data)

}

func trigger(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	data := job.JobTriggerFile(project, job_id)

	if data == "" {
		return c.NoContent(http.StatusNotFound)
	} else {
		return c.String(http.StatusOK, data)
	}

}
