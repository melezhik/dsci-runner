package main

import (

	// dsci related deps
	"database/sql"
	"dsci_runner/job"
	"dsci_runner/types"
	"dsci_runner/utils"
	"dsci_runner/git"
	"dsci_runner/html"
	"bufio"
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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"net/http/cgi"
	"bytes"
	"sort"
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

var AppConfig types.AppConfig

func main() {

	if len(os.Args) > 1 {
		// Run CLI logic
		runCLI()
		return
	}

  err := os.MkdirAll(repoRoot, 0755)

  if err != nil {
    log.Fatalf("main: error creating directory %s: %s", repoRoot, err)
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
    AppConfig.GitPathToHttpBackend = "/usr/lib/git-core/git-http-backend"
  }
  
  if AppConfig.GitServerAddress == "" {
    AppConfig.GitServerAddress = "http://localhost:8080"
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
	e.GET("/", list_repos)
	e.GET("/repo/:repo", list_files)
	e.GET("/repo/:repo/file/:file", dump_file)
	e.GET("/repo/:repo/dir/:dir", list_files)
	e.GET("/builds", builds)
	e.POST("/queue", queue_job)
	e.POST("/stash", put_job_stash)
	e.GET("/stash/:project/:key", get_job_stash)
	e.GET("/status/:project/:key", status)
	e.GET("/report/ui/:project/:key", report_ui)
	e.GET("/report/raw/:project/:key", report)
	e.GET("/trigger/:project/:key", trigger)
	e.GET("/livebuilds", livebuilds)

	// =========================================================================
	// Хендлер для Git Clone / Push / Fetch поверх HTTP
	// =========================================================================
	// Захватываем любые эндпоинты, заканчивающиеся на .git или содержащие его в пути


	gitBackendPath := AppConfig.GitPathToHttpBackend

	gitRoot, _ := filepath.Abs(repoRoot)

	cgiHandler := &cgi.Handler{
	    Path: gitBackendPath,
	    Env: []string{
	        "GIT_PROJECT_ROOT=" + gitRoot,
	        "GIT_HTTP_EXPORT_ALL=1",
	        "REMOTE_USER=foobarbaz",
	    },
	    Dir: gitRoot,
	}

	
	e.Any("/*", func(c *echo.Context) error {

		// 1. Capture the Echo v5 Response (which is a raw http.ResponseWriter)
		originalWriter := c.Response()

		// 2. Wrap it with our status interceptor
		sw := &git.StatusWriter{ResponseWriter: originalWriter, Status: 200}

		// 3. Re-wrap the interceptor using Echo v5's layout wrapper
		// Note: Echo v5's NewResponse expects (http.ResponseWriter, *slog.Logger) 
		// If using an older v5 beta, it might expect (http.ResponseWriter, *echo.Echo)
		newEchoResponse := echo.NewResponse(sw, e.Logger)

		// 1. Read the body bytes safely
		req := c.Request()
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
    data := []git.GitUpdate{}
    if req.Method == http.MethodPost {
      // Check path suffix for receive-pack (push operation)
      isReceivePath := strings.HasSuffix(req.URL.Path, "/git-receive-pack")
      isCorrectType := req.Header.Get("Content-Type") == "application/x-git-receive-pack-request"
        if isReceivePath && isCorrectType {
  		    data = git.HandleGitPush(bodyBytes)
        }
    }
		// log.Printf("HHHHH")
		// 2. Restore the body for both Echo and the CGI handler
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		cgiHandler.ServeHTTP(newEchoResponse, c.Request())

		// 5. Evaluate the captured status code
		if sw.Status >= 200 && sw.Status < 300 {
			log.Printf("CGI Request Succeeded with status: %d", sw.Status)
			if req.Method == http.MethodPost {
				// Check path suffix for receive-pack (push operation)
				isReceivePath := strings.HasSuffix(req.URL.Path, "/git-receive-pack")
				// Check specific Smart HTTP content type for pushes
				isCorrectType := req.Header.Get("Content-Type") == "application/x-git-receive-pack-request"
				if isReceivePath && isCorrectType {
					// 2. Get the full wildcard path matches
					wildcardPath := c.Param("*") // Returns "company/team/project.git/git-receive-pack"

					// 3. Strip the git-receive-pack suffix
					repoPath := strings.TrimSuffix(wildcardPath, "/git-receive-pack")

					// 4. Strip the trailing .git extension to get the clean repository path identifier
					repoName := strings.TrimSuffix(repoPath, ".git") // Returns "company/

					log.Printf("git push to %s, data: %s\n", repoName, data)

					var q types.JobRequest
					now := time.Now()
					q.Config.Project = "dsci"
					q.Config.JobId = strconv.FormatInt(now.Unix(), 10)
					msg := "ci"
					q.Config.Description = fmt.Sprintf("%s | %s", data[0].NewCommit, msg)
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
						"cr=%s,ref=%s,repo_full_name=%s,sha=%s,scm=%s,message=%s,DsciFeedbackUrl=%s,DsciAgentImage=%s%s%s",
					AppConfig.DsciContainerRuntime,
						data[0].RefName,
						repoName,
						data[0].NewCommit,
						fmt.Sprintf("http://localhost:8080/%s.git",repoName),
						msg,
						AppConfig.DsciFeedbackUrl,
						AppConfig.DsciAgentImage,
						skip_bootstrap,
						allow_localhost_mode,
					)
				
					q.Trigger.Sparrowdo.NoSudo = true
				
					q.Trigger.Sparrowdo.Localhost = true
				
					dat := job.GetSparkyScenarioFile("dsci", "sparrowfile")
				
					q.Sparrowfile = dat
				
					job.JobQueueFs(q,AppConfig.DsciContainerRuntime)
				
					log.Printf("job quedued: %s\n", q.Config.JobId)

				}
			}
		} else {
			log.Printf("CGI Request Failed with status: %d", sw.Status)
		}		
		return nil
	})


	// Start server
	if err := e.Start("0.0.0.0:8080"); err != nil {
		slog.Error("failed to start server", "error", err)
	}
}

// Handlers

// Main handlers

func list_repos(c *echo.Context) error {
	// Read the root directory contents
	dir, _ := filepath.Abs(repoRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("list_repos ReadDir error: %s", err)
	}

	data := ""

	for _, entry := range entries {
		// Output the name and whether it is a directory
		if entry.IsDir() {
			data += fmt.Sprintf(
				"<a href=\"/repo/%s\">%s</a>\n",
				entry.Name(),
				entry.Name(),
			)
		}
	}

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
	<div class="container">			
	  <div>
        <p class="title">DSCI Git Repos</p>
         <hr>
        <pre>%s</pre>
      </div>
    </div>
 </body>
</html>`, html.Header(), html.NavBar(""), data))
}

func list_files(c *echo.Context) error {
	
	dname := utils.GitCacheRootDir() + "/" + c.Param("repo")

	err := os.MkdirAll(dname, 0755)

	if err != nil {
		log.Fatalf("list_files: error creating directory %s: %s", dname, err)
	}

	log.Printf("list_files: creating git chache dir: %s OK", dname)

	// extract files from git bare bones

	path, _ := filepath.Abs(
		fmt.Sprintf("%s/%s",
			repoRoot,
			c.Param("repo"),
		),
	)

	cmd := exec.Command(
		"git",
		("--git-dir=" + path),
		 "--work-tree=.",
		"checkout",
		"-f", 
		"HEAD",
	)

	cmd.Dir = dname

	output, err := cmd.CombinedOutput() // Run the command and wait for completion

	if err != nil {
		log.Printf("list_files: extract files failed with: %s\n", output)
	}

	log.Printf("list_files: extract files OK: %s", output)

	prefix := ""
	if c.Param("dir") != "" {
		dname = dname + "/" + c.Param("dir")
		prefix = c.Param("dir") + "/"
	}
	entries, err := os.ReadDir(dname)

	if err != nil {
		log.Fatalf("list_files ReadDir error: %s", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		isDirI := entries[i].IsDir()
		isDirJ := entries[j].IsDir()

		// If one is a directory and the other is not
		if isDirI != isDirJ {
			return isDirI // puts true (directories) first
		}

		// Otherwise, sort alphabetically by name
		return entries[i].Name() < entries[j].Name()
	})

	data := ""

	for _, entry := range entries {
		// Output the name and whether it is a directory
		if entry.IsDir() {
			data += fmt.Sprintf(
				"<a href=\"/repo/%s/dir/%s%s\">[%s]</a>\n",
				c.Param("repo"),
				prefix,
				entry.Name(),
				entry.Name(),
			)
		} else {
			data += fmt.Sprintf(
				"<a href=\"/repo/%s/file/%s%s\">%s</a>\n",
				c.Param("repo"),
				prefix,
				entry.Name(),
				entry.Name(),
			)
		}
	}

	uplink := ""

	if c.Param("dir") == "" {

		uplink = fmt.Sprintf(
			`%s`,
			c.Param("repo"),
		)

	} else {

		parts := strings.Split(c.Param("dir"), "/")
		cdir := parts[len(parts)-1]
	
		if len(parts) > 0 {
			parts = parts[:len(parts)-1]
		}

		upper_dir := strings.Join(parts, "/")

		if upper_dir == "" {
			uplink = fmt.Sprintf(
				`<a href="/repo/%s">%s</a> | %s`,
				c.Param("repo"),
				c.Param("repo"),
				cdir,
			)
		} else {

			uplink = fmt.Sprintf(
				`<a href="/repo/%s">%s</a> | <a href="/repo/%s/dir/%s">%s</a>/%s`,
				c.Param("repo"),
				c.Param("repo"),
				c.Param("repo"),
				upper_dir,
				upper_dir,
				cdir,
			)
		}
	
	}


	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  <div>
        <p class="title">DSCI Git Repo Files | %s</p>
		<div class="field has-addons">
		<div class="control is-expanded">
			<input class="input" type="text" id="copyInput" value="git clone %s/%s" readonly>
		</div>
		<div class="control">
			<button class="button is-info" onclick="copyText()">Copy</button>
		</div>
		</div>

		<script>
		function copyText() {
			var copyText = document.getElementById("copyInput");
			copyText.select();
			navigator.clipboard.writeText(copyText.value);
		}
		</script>
         <hr>
        <pre>%s</pre>
      </div>
    </div>
 </body>
</html>`, 
html.Header(), 
html.NavBar(""),
uplink,
AppConfig.GitServerAddress,
c.Param("repo"),
data,
))
}


func dump_file (c *echo.Context)  error {

	repo := c.Param("repo")

	dname := utils.GitCacheRootDir() + "/" + repo

	file := c.Param("file")

	content, err := os.ReadFile(dname + "/" + file)

	if err != nil {
		return c.String(
			http.StatusNotFound, 
			fmt.Sprintf(
				"dump file error, repo: %s, file: %s, error: %s", 
				repo, file, err,
			),
		)
    }
	code := html.CodeToHtml(string(content))

	parts := strings.Split(file, "/")

	file_short_name := parts[len(parts)-1]

	// 2. Remove the last element by re-slicing
	if len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	
	localdir := strings.Join(parts, "/")
	link := ""
	if localdir == "" {
		link = fmt.Sprintf(
			`<a href="/repo/%s">%s</a>`,
			repo,
			repo,
		)
	} else {
		link = fmt.Sprintf(
			`<a href="/repo/%s">%s</a> | <a href="/repo/%s/dir/%s">%s/</a>`,
			repo,
			repo,
			repo,
			localdir,
			localdir,
		)
	}


	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  <div>
        <p class="title">%s | %s</p>
         <hr>
        %s
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(""),
	link, 
	file_short_name, 
	code,
	),
)

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
</html>`, html.Header(), html.NavBar(""), project, job_id, string(htmlOutput)))
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
		fmt.Sprintf(html.LiveBuilds(), html.Header(), html.NavBar("")),
	)
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
