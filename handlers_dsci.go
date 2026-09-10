// DSCI CI related handlers
package main

import (
	"dsci_runner/html"
	"dsci_runner/job"
	"dsci_runner/utils"
	"fmt"
	"github.com/labstack/echo/v5"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"strings"

	"database/sql"
	"dsci_runner/types"
	"encoding/json"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robert-nix/ansihtml"
	"golang.org/x/net/websocket"
	"io"
	"time"
)

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

	job.JobQueueFs(r, AppConfig.DsciContainerRuntime)

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

func report_ui(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	data := job.Report(project, job_id)

	data = strings.ReplaceAll(data, "\r\n", "\n")

	htmlOutput := ansihtml.ConvertToHTML([]byte(data))

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
      <div>
        <p class="title">DSCI Report: %s@%s</p>
        <hr>
        <pre>%s</pre>
      </div>
    </div>
</body>
</html>`, html.Header(), html.NavBar(user_is_logged(c)), project, job_id, string(htmlOutput)))
}

func report_ui2(c *echo.Context) error {

	project := c.Param("project")

	build_id := c.Param("build_id")

	data := job.ReportByBuildId(project, build_id)

	data = strings.ReplaceAll(data, "\r\n", "\n")

	htmlOutput := ansihtml.ConvertToHTML([]byte(data))

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
      <div>
        <p class="title">DSCI Report: %s@%s</p>
        <hr>
        <pre>%s</pre>
      </div>
    </div>
</body>
</html>`, html.Header(), html.NavBar(user_is_logged(c)), project, build_id, string(htmlOutput)))
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

func livebuilds(c *echo.Context) error {

	db, err := sql.Open("sqlite3", utils.SparkyDbFile())
	defer db.Close()

	if err != nil {
		log.Fatalf("livebuilds: error opening db file: %s", err)
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		for {
			// Write
			builds := job.Builds(db)
			jsonData, _ := json.MarshalIndent(builds, "", "  ") // Use MarshalIndent for pretty printing

			_ = websocket.Message.Send(ws, string(jsonData))
			// if err := websocket.Message.Send(ws, string(jsonData)); err != nil {
			// 	log.Printf("livebuilds: failed to write WS message: %s", "error", err)
			// }
			//log.Printf("livebuilds: send data: %s", string(jsonData))
			//log.Printf("\n===\nlivebuilds: sleep for 10 second\n===\n")
			time.Sleep(10 * time.Second)

			//   // Read
			//   msg := ""
			//   if err := websocket.Message.Receive(ws, &msg); err != nil {
			//    c.Logger().Error("failed to read WS message", "error", err)
			//   }
			//   fmt.Printf("%s\n", msg)

		}
	}).ServeHTTP(c.Response(), c.Request())
	return nil
}

func builds(c *echo.Context) error {
	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(html.LiveBuilds(), html.Header(), html.NavBar(user_is_logged(c))),
	)
}

func put_job_file(c *echo.Context) error {

	data := c.Request().Body

	defer data.Close()

	bytesWritten, err := job.PutJobFile(
		c.Param("project"),
		c.Param("job_id"),
		c.Param("filename"),
		data,
	)
	if err != nil {
		return c.String(http.StatusInternalServerError, "handleBlobUpload: job.PutJobFile error")
	}
	// Возвращаем успешный ответ клиенту
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "File succssfully created",
		"size_bytes": bytesWritten,
		"error":      "",
	})
}

func get_job_file(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("job_id")

	filename := c.Param("filename")

	data, err := job.GetJobFile(
		project,
		job_id,
		filename,
	)

	if err != nil {
		return c.String(http.StatusInternalServerError, "get_job_file: job.GetJobFile error")
	}

	return c.Blob(http.StatusOK, "application/octet-stream", data)

}
