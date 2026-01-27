package main

import (
	//"errors"
	"dsci_runner/job"
	"dsci_runner/types"
	"encoding/json"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"io"
	"log"
	"log/slog"
	"net/http"
)

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLogger()) // use the RequestLogger middleware with slog logger
	e.Use(middleware.Recover())       // recover panics as errors for proper error handling

	// Routes
	e.GET("/", hello)

	// Routes
	e.POST("/queue", queue_job)

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
		log.Printf("error unmarshaling JSON: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	}

	// if err != nil {
	//       return echo.NewHTTPError(http.StatusBadRequest, "Invalid JSON body")
	// }

	//log.Printf("data: %v", r)
	// Process the data (e.g., save to a database)

	job.JobQueueFs(r)

	return c.String(http.StatusOK, "job queued")

}
