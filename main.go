package main

import (

	// dsci related deps

	"bufio"
	"database/sql"
	"dsci_runner/job"
	"dsci_runner/types"
	"dsci_runner/utils"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pelletier/go-toml/v2"
	"github.com/robert-nix/ansihtml"
	"golang.org/x/net/websocket"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"net/http/cgi"
)

// Git related constants
// TODO: move to ~/.dsci.toml

const (
	repoRoot     = ".repositories"
	sshAddr      = ":2222"
)

//go:embed docker
var staticFiles embed.FS

//go:embed podman
var staticFiles11 embed.FS

//go:embed common
var staticFiles12 embed.FS

//go:embed public
var staticFiles2 embed.FS

var AppConfig types.AppConfig

func main() {

	if len(os.Args) > 1 {
		// Run CLI logic
		runCLI()
		return
	}
	path := utils.DsciConfigFile()

	dat, err := os.ReadFile(path)

	if err != nil {
		log.Fatalf("main: Error reading file: %s : %s", path, err)
	}

	err = toml.Unmarshal(dat, &AppConfig)

	if err != nil {
		log.Fatalf("main: Error parsing toml config: %s : %s", path, err)
	}

  if AppConfig.DsciContainerRuntime == "" {
    AppConfig.DsciContainerRuntime = "docker"
  }

  if AppConfig.GitPathToHttpBackend == "" {
    AppConfig.GitPathToHttpBackend = "/usr/libexec/git-core/git-http-backend"
  }

	go func() {
    for {
      startJobDispatcher()
      time.Sleep(10 * time.Second)
      log.Printf("JobDispatcher stopped, restarting it with startJobDispatcher() ...")
    }
  }()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLogger()) // use the RequestLogger middleware with slog logger
	e.Use(middleware.Recover())       // recover panics as errors for proper error handling

	// Routes
	e.GET("/", builds)
	e.POST("/queue", queue_job)
	e.POST("/stash", put_job_stash)
	e.GET("/stash/:project/:key", get_job_stash)
	e.POST("/forgejo_hook", forgejo_hook)
	e.GET("/status/:project/:key", status)
	e.GET("/report/ui/:project/:key", report_ui)
	e.GET("/report/raw/:project/:key", report)
	e.GET("/trigger/:project/:key", trigger)
	e.GET("/livebuilds", livebuilds)
	e.GET("/builds", builds)

	// =========================================================================
	// Хендлер для Git Clone / Push / Fetch поверх HTTP
	// =========================================================================
	// Захватываем любые эндпоинты, заканчивающиеся на .git или содержащие его в пути
	e.Any("/*", gitHTTPBackendHandler())


	// Start server
	if err := e.Start("0.0.0.0:8080"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}

// Handlers

// Git related handlers

func gitHTTPBackendHandler() echo.HandlerFunc {

	gitBackendPath := AppConfig.GitPathToHttpBackend

	gitRoot, _ := filepath.Abs(repoRoot)

	cgiHandler := &cgi.Handler{
	    Path: gitBackendPath,
	    Env: []string{
	        "GIT_PROJECT_ROOT=" + gitRoot,
	        "GIT_HTTP_EXPORT_ALL=1",
	        "REMOTE_USER=foobar",
	    },
	    Dir: gitRoot,
	}

	return echo.WrapHandler(cgiHandler)
}

// DSCI CI related handlers

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

	job.JobQueueFs(r,AppConfig.DsciContainerRuntime)

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

	var host *url.URL

	forgejo_host := "127.0.0.1"

	if AppConfig.ForgejoHost != ""  {
		host, err = url.Parse(AppConfig.ForgejoHost)
		if err != nil {
			log.Fatalf(
				"forgejo_hook, error domain parsing AppConfig.ForgejoHost: %s, %s",
				AppConfig.ForgejoHost,
				err,
			)
		}
		forgejo_host = host.Hostname()
	} 

	clone_host := "127.0.0.1"
	host, err = url.Parse(r.Repository.CloneUrl)
	if err != nil {
		log.Fatalf(
			"forgejo_hook, error domain parsing r.Repository.CloneUrl: %s, %s",
			r.Repository.CloneUrl,
			err,
		)
	} else {
		clone_host = host.Hostname()
	}

	log.Printf(
		"forgejo_hook Proccess request. clone_host: %s | forgejo_host: %s\n",
		clone_host,
		forgejo_host,
	)

	if clone_host != forgejo_host {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf(
				"forgejo_hook. Invalid request. clone_host: %s != forgejo_host: %s",
				clone_host,
				forgejo_host,
			),
		)
	}
	// job.PutJobStash(r.Config.Project,r.Config.JobId,r.Data)

	var q types.JobRequest
	now := time.Now()
	q.Config.Project = "dsci"
	q.Config.JobId = strconv.FormatInt(now.Unix(), 10)
	q.Config.Description = fmt.Sprintf("%s | %s", r.Sha, r.HeadCommit.Message)
	skip_bootstrap := ""
	allow_localhost_mode := ""
	if AppConfig.DsciAgentSkipBootstrap == true {
		skip_bootstrap = ",DsciAgentSkipBootstrap"
	}
	if len(AppConfig.DsciAllowLocalhostModeRepos) > 0 {
		repos := strings.Join(AppConfig.DsciAllowLocalhostModeRepos,":")
		allow_localhost_mode = fmt.Sprintf(",DsciAllowLocalhostModeRepos=%s",repos)
	}
	q.Trigger.Sparrowdo.Tags = fmt.Sprintf(
		"cr=%s,ref=%s,repo_full_name=%s,sha=%s,scm=%s,message=%s,ForgejoApiToken=%s,ForgejoHost=%s,DsciFeedbackUrl=%s,DsciAgentImage=%s%s%s",
    AppConfig.DsciContainerRuntime,
		r.Ref,
		r.Repository.FullName,
		r.Sha,
		r.Repository.CloneUrl,
		r.HeadCommit.Message,
		AppConfig.ForgejoApiToken,
		AppConfig.ForgejoHost,
		AppConfig.DsciFeedbackUrl,
		AppConfig.DsciAgentImage,
		skip_bootstrap,
		allow_localhost_mode,
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

	job.JobQueueFs(q,AppConfig.DsciContainerRuntime)

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

func report_ui(c *echo.Context) error {

	project := c.Param("project")

	job_id := c.Param("key")

	data := job.Report(project, job_id)

	data = strings.ReplaceAll(data, "\r\n", "\n")

	htmlOutput := ansihtml.ConvertToHTML([]byte(data))

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`<html data-theme="dark">
  <head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.15.0/dist/katex.min.css">
    <title>DSCI Jobs</title>
  </head>
  <body>
    <section class="hero">
      <div class="hero-body">
        <p class="title">DSCI Report: %s@%s</p>
        <hr>
        <pre>%s</pre>
      </div>
    </section>
</body>
</html>`,
			project,
			job_id,
			string(htmlOutput),
		),
	)

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

	content, err := fs.ReadFile(staticFiles2, "public/builds.html")
	if err != nil {
		log.Fatalf("builds: error reading public/builds.html: %s", err)
	}
	return c.HTML(http.StatusOK, string(content))
}

func startJobDispatcher() {

	log.Printf("startJobDispatcher: start")

	dname, err := os.MkdirTemp("", "dsci_container")

	defer os.RemoveAll(dname)

	if err != nil {
		log.Fatalf("startJobDispatcher: error creating temp dir: %s", err)
	}

	log.Printf("startJobDispatcher: creating temp dir: %s OK", dname)

	// Dockerfile

	var content []byte

  if AppConfig.DsciContainerRuntime == "docker" {
	  content, err = fs.ReadFile(staticFiles, "docker/Dockerfile")
    if err != nil {
      log.Fatalf("startJobDispatcher: error reading docker/Dockerfile: %s", err)
    }
  } else {
	  content, err = fs.ReadFile(staticFiles11, "podman/Dockerfile")
    if err != nil {
      log.Fatalf("startJobDispatcher: error reading podman/Dockerfile: %s", err)
    }
  }

	fname := filepath.Join(dname, "Dockerfile")

	err = os.WriteFile(fname, []byte(content), 0644)

	if err != nil {
		log.Fatalf("startJobDispatcher: error writting to %s/Dockerfile: %s", dname, err)
	}

	log.Printf("startJobDispatcher: writting %s/Dockerfile OK", dname)

	// sparrowfile

  content, err = fs.ReadFile(staticFiles12, "common/sparrowfile")

  if err != nil {
		  log.Fatalf("startJobDispatcher: error reading common/sparrowfile: %s", err)
  }

	fname = filepath.Join(dname, "sparrowfile")

	err = os.WriteFile(fname, []byte(content), 0644)

	if err != nil {
		log.Fatalf("startJobDispatcher: error writting to %s/sparrowfile: %s", dname, err)
	}

	log.Printf("startJobDispatcher: writting %s/sparrowfile OK", dname)

	// sparky.yaml

	content, err = fs.ReadFile(staticFiles12, "common/sparky.yaml")

	if err != nil {
		log.Fatalf("startJobDispatcher: error reading common/sparky.yaml: %s", err)
	}

	fname = filepath.Join(dname, "sparky.yaml")

	err = os.WriteFile(fname, []byte(content), 0644)

	if err != nil {
		log.Fatalf("startJobDispatcher: error writting to %s/sparky.yaml: %s", dname, err)
	}

	log.Printf("startJobDispatcher: writting %s/sparky.yaml OK", dname)

	// build.sh

  if AppConfig.DsciContainerRuntime == "docker" {
	  content, err = fs.ReadFile(staticFiles, "docker/build.sh")
	  if err != nil {
		  log.Fatalf("startJobDispatcher: error reading docker/build.sh: %s", err)
	  }
  } else {
    content, err = fs.ReadFile(staticFiles11, "podman/build.sh")
    if err != nil {
      log.Fatalf("startJobDispatcher: error reading podman/build.sh: %s", err)
    }
  }

	fname = filepath.Join(dname, "build.sh")

	err = os.WriteFile(fname, []byte(content), 0644)

	if err != nil {
		log.Fatalf("startJobDispatcher: error writting to %s/build.sh: %s", dname, err)
	}

	log.Printf("startJobDispatcher: writting %s/build.sh OK", dname)

	// run container build

	cmd := exec.Command("sh", "build.sh")

	cmd.Dir = dname

	output, err := cmd.CombinedOutput() // Run the command and wait for completion

	if err != nil {
		log.Printf("startJobDispatcher: build.sh failed with: %s\n", output)
	}

	log.Printf("startJobDispatcher: build.sh OK: %s", output)

  cmd = exec.Command(
    AppConfig.DsciContainerRuntime,
    "exec",
    "dsci-dispatch",
    "sparkyd",
  )

	stdoutPipe, err := cmd.StdoutPipe()

	if err != nil {
		log.Fatalf("startJobDispatcher(sparkyd): Failed to create stdout pipe: %v", err)
	}

	stderrPipe, err := cmd.StderrPipe()

	if err != nil {
		log.Fatalf("startJobDispatcher(sparkyd): Failed to create stderr pipe: %v", err)
	}

	// Read stdout in a goroutine
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			fmt.Println("sparkyd_stdout",scanner.Text())
		}
	}()

	// Read stderr in a goroutine
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			fmt.Println("sparkyd_stderr",scanner.Text())
		}
	}()

	// Start the infinite/long-running process
	if err := cmd.Start(); err != nil {
		log.Fatalf("startJobDispatcher(sparkyd): Failed to start command: %v", err)
	}

	// Wait for the command to finish (if it ever does)
	if err := cmd.Wait(); err != nil {
		log.Printf("startJobDispatcher(sparkyd): Command finished with error: %v", err)
	}
}

func runCLI() {

	actPtr := flag.String("action", "create-secret", "a string")
	//numbPtr := flag.Int("numb", 42, "an int")
	adminPtr := flag.Bool("admin", false, "a bool")

	flag.Parse()
	//fmt.Printf("run cli: %s %s\n",*adminPtr,*actPtr)

	if *adminPtr == true {
		if *actPtr == "create-secret" {
			reader := bufio.NewReader(os.Stdin)
			default_value := "demo/demo-php/password"
			fmt.Printf("path: (%s) ", default_value)
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Fatalf("runCLI, error reading input:", err)
			}
			secret_path := strings.TrimSpace(input)
			if secret_path == "" {
				secret_path = default_value
			}
			pattern := `^[\w\-]+\/[\w\-\_]+\/[\w\-\_]+$`
			if match, _ := regexp.MatchString(pattern, secret_path); match == false {
				log.Fatalf("runCLI, secret_path should match: %s", pattern)
			}

			default_value = "12345"

			fmt.Printf("value: (%s) ", default_value)

			input, err = reader.ReadString('\n')

			if err != nil {
				log.Fatalf("runCLI, error reading input:", err)
			}

			secret_value := strings.TrimSpace(input)

			if secret_value == "" {
				secret_value = default_value
			}

			pattern = `^\S+$`

			if match, _ := regexp.MatchString(pattern, secret_value); match == false {
				log.Fatalf("runCLI, secret_value should match: %s", pattern)
			}

			dir := utils.DsciRootDir()

			slice := strings.Split(secret_path, "/")
			secret_name := slice[len(slice)-1]
			slice = slice[:len(slice)-1]

			repo := strings.Join(slice, "/")

			dir = fmt.Sprintf("%s/.secrets/%s", dir, repo)

			err = os.MkdirAll(dir, 0755)

			if err != nil {
				log.Fatalf("Error creating directory %s: %s", dir, err)
			}

			fname := filepath.Join(dir, secret_name)

			err = os.WriteFile(fname, []byte(secret_value), 0644)

			if err != nil {
				log.Fatalf("runCLI: error writting to file %s: %s", fname, err)
			}
		}
	}
}
