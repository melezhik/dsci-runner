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
	"log"
	// "log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	//"strconv"
	"strings"
	"time"
	"net/http/cgi"
	"bytes"
	"sort"
	"errors"
	"math/rand/v2"
  html_utils "html"
	go_git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	//"github.com/go-git/go-git/v6/storage/memory"
	//"github.com/go-git/go-billy/v6/memfs"
	go_git_http "github.com/go-git/go-git/v6/plumbing/transport/http"
	go_git_client "github.com/go-git/go-git/v6/plumbing/client"

	"os/signal"
	"syscall"
	"context"
)

// Git related constants
// TODO: move to ~/.dsci.toml

const (
	repoRoot     = ".repositories"
	sshAddr      = ":2222"
)

//go:embed common
var staticFiles12 embed.FS

var AppConfig types.AppConfig

type ChangeFilePayload struct {
	Code  string `form:"code"`
	Message  string `form:"message"`
}

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

	// set configuration defaults
	if AppConfig.DsciContainerRuntime == "" {
		AppConfig.DsciContainerRuntime = "docker"
	}

	if AppConfig.GitPathToHttpBackend == "" {
		AppConfig.GitPathToHttpBackend = "/usr/lib/git-core/git-http-backend"
	}
	
	if AppConfig.GitServerAddress == "" {
		AppConfig.GitServerAddress = "http://localhost:8080"
	}

	if AppConfig.GitAuthUser == "" {
		AppConfig.GitAuthUser = "dsci"
	}

	if AppConfig.GitAuthPassword == "" {
		AppConfig.GitAuthPassword = "dsci"
	}

	// Echo instances - public and private

	// public
	e1 := echo.New()

	// private
	e2 := echo.New()

	// Middleware
	e1.Use(middleware.RequestLogger()) // use the RequestLogger middleware with slog logger
	e1.Use(middleware.Recover())       // recover panics as errors for proper error handling
	e2.Use(middleware.RequestLogger()) // use the RequestLogger middleware with slog logger
	e2.Use(middleware.Recover())       // recover panics as errors for proper error handling

	// public routes
	e1.GET("/login", login_form)
	e1.POST("/login", create_session)
	e1.GET("/logout", drop_session)
	e1.GET("/", list_repos)
	e1.GET("/repo/:repo", list_files)
	e1.GET("/repo/:repo/file/:file", dump_file)
	e1.POST("/repo/:repo/file/:file", change_file)
	e1.GET("/repo/:repo/file_edit/:file", edit_file)
	e1.GET("/repo/:repo/dir/:dir", list_files)
	e1.GET("/builds", builds)
	e1.POST("/repo/:repo/build",manual_build)
	e1.GET("/livebuilds", livebuilds)

	e1.GET("/report/ui/:project/:key", report_ui)
	e1.GET("/report/ui2/:project/:build_id", report_ui2)
	e1.GET("/report/raw/:project/:key", report)

	// private routes
	e2.POST("/queue", queue_job)
	e2.POST("/stash", put_job_stash)
	e2.PUT("/file/project/:project/job/:job_id/filename/:filename", put_job_file)
	e2.GET("/file/:project/:job_id/:filename", get_job_file)

	
	e2.GET("/stash/:project/:key", get_job_stash)
	e2.GET("/status/:project/:key", status)
	e2.GET("/report/ui/:project/:key", report_ui)
	e2.GET("/trigger/:project/:key", trigger)

	e2.GET("/report/raw/:project/:key", report)

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

	
	e1.Any("/*", func(c *echo.Context) error {

		// 1. Capture the Echo v5 Response (which is a raw http.ResponseWriter)
		originalWriter := c.Response()

		// 2. Wrap it with our status interceptor
		sw := &git.StatusWriter{ResponseWriter: originalWriter, Status: 200}

		// 3. Re-wrap the interceptor using Echo v5's layout wrapper
		// Note: Echo v5's NewResponse expects (http.ResponseWriter, *slog.Logger) 
		// If using an older v5 beta, it might expect (http.ResponseWriter, *echo.Echo)
		newEchoResponse := echo.NewResponse(sw, e1.Logger)

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
				// 1. Extract the credentials from the request headers
				username, password, ok := c.Request().BasicAuth()

				// 2. Validate existence and match your expected credentials
				if !ok || username != AppConfig.GitAuthUser || password != AppConfig.GitAuthPassword {
					// 3. Set the WWW-Authenticate header so the browser/client prompts for credentials
					c.Response().Header().Set("WWW-Authenticate", `Basic realm="DSCI Git Subscribed Area"`)
					
					// 4. Return 401 Unauthorized error
					return echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
				}
				data = git.HandleGitPush(bodyBytes)
			}
		}
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
					// Get the full wildcard path matches
					wildcardPath := c.Param("*") // Returns "company/team/project.git/git-receive-pack"

					// Strip the git-receive-pack suffix
					repoPath := strings.TrimSuffix(wildcardPath, "/git-receive-pack")

					// Strip the trailing .git extension to get the clean repository path identifier
					repoName := strings.TrimSuffix(repoPath, ".git")

					log.Printf("git push to %s, data: %s\n", repoName, data)

					repo_dir := gitRoot + "/" + repoName + ".git"
					repo, err := go_git.PlainOpen(repo_dir)

					if err != nil {
						log.Fatalf("Failed to open repository: %v", err)
					}

					// Parse the string SHA into a plumbing.Hash object
					hash := plumbing.NewHash(data[0].NewCommit)

					// Retrieve the commit object from the repository
					commit, err := repo.CommitObject(hash)
					if err != nil {
						log.Fatalf("Failed to find commit %s: %v", data[0].NewCommit, err)
					}

					// Print the full commit message
					fmt.Println("--- Commit Message ---")
					fmt.Println(commit.Message)
					fmt.Println("----------------------")

					// Bonus: You can also easily access metadata
					fmt.Printf("Author: %s <%s>\n", commit.Author.Name, commit.Author.Email)
					fmt.Printf("Date:   %s\n", commit.Author.When)

					msg := fmt.Sprintf(
						"%s by %s",
						commit.Message,
						commit.Author.Email,
					)
					shortSHA := data[0].NewCommit[:7]
					job_description := fmt.Sprintf("%s: %s | %s", repoName, shortSHA, msg)
					// JobQueue(app_cfg types.AppConfig, job_id string, msg string, repo string, ref, sha string,description string)
					job.JobQueue(
						AppConfig,
						shortSHA,
						msg,
						repoName,
						data[0].RefName,
						shortSHA,
						job_description,
					)
				}
			}
		} else {
			log.Printf("CGI Request Failed with status: %d", sw.Status)
		}
		return nil
	})


	server1 := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: e1,
	}

	server2 := &http.Server{
		Addr:    "127.0.0.1:8181",
		Handler: e2,
	}

	// Канал для перехвата критических ошибок запуска портов
	errChan := make(chan error, 2)

	// Запуск первого сервера в фоне (неблокирующий)
	go func() {
		log.Println("Start public dsci server, port :8080...")
		if err := server1.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("public dsci server error: %w", err)
		}
	}()

	// Запуск второго сервера в фоне (неблокирующий)
	go func() {
		log.Println("Start ptivate dsci server, port :8181...")
		if err := server2.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("private dsci server error: %w", err)
		}
	}()

	// Канал для отслеживания сигналов завершения от ОС (Ctrl+C, SIGTERM)
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Блокируем главный поток до первой ошибки или сигнала остановки
	select {
	case err := <-errChan:
		log.Printf("Critical server(s) run error: %v", err)
	case sig := <-stopChan:
		log.Printf("Signal recieved %v. Start Graceful Shutdown...", sig)
	}

	// Выделяем 2 секунд на плавное закрытие активных соединений
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Останавливаем оба сервера параллельно в горутинах
	go func() {
		if err := server1.Shutdown(ctx); err != nil {
			log.Printf("Error when try to stop public dsci server: %v", err)
		}
	}()

	go func() {
		if err := server2.Shutdown(ctx); err != nil {
			log.Printf("Error when try to stop private dsci server: %v", err)
		}
	}()

	// Ждем завершения таймаута контекста
	<-ctx.Done()
	log.Println("Both dsci servers successfully stopped.")
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
</html>`, html.Header(), html.NavBar(user_is_logged(c)), data))
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

	gitRoot, _ := filepath.Abs(repoRoot)

	repo_dir := gitRoot + "/" + c.Param("repo")

	repo, err := go_git.PlainOpen(repo_dir)

	if err != nil {
		log.Fatalf("Failed to open repository: %v", err)
	}

	_, err = repo.Head()

	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		// No references mean it's an empty repo
		return c.HTML(
			http.StatusOK,
			fmt.Sprintf(
				`%s %s
		<div class="container">
			  <div>
				<p class="title">DSCI Git Repo Files</p>
				<div class="field has-addons">
					<div class="control is-expanded">
						<input class="input" type="text" id="copyInput" value="git clone %s/%s" readonly>
					</div>
					<div class="control">
						<button class="button is-info" id="copyBtn">Copy</button>
					</div>
				</div>
			<hr>	
			<pre>no files - empty repository</pre>
			</div>
		</div>
	 %s
	 </body>
	</html>`, 
		html.Header(), 
		html.NavBar(user_is_logged(c)),
		AppConfig.GitServerAddress,
		c.Param("repo"),
		html.CopyPasteButtonScript(),
	))		
	}

	ref, _ := repo.Head()

	// 3. Parse the string SHA into a plumbing.Hash object
	hash := ref.Hash()

	// 4. Retrieve the commit object from the repository
	commit, err := repo.CommitObject(hash)
	if err != nil {
		log.Fatalf("Failed to find commit %s: %v", "HEAD", err)
	}

	shortSHA := commit.Hash.String()[:7]

	state := job.JobState("dsci", shortSHA)

	state_badge := `<span class="tag is-white is-medium">Unknown</span>`

	if state == "0" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-warning is-medium"><a href="/report/ui/dsci/%s">Running</a></span>`,
			shortSHA,
		)
	}
	if state == "1" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-success is-medium"><a href="/report/ui/dsci/%s">Pass</a></span>`,
			shortSHA,
		)
	}
	if state == "-1" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-danger" is-medium><a href="/report/ui/dsci/%s">Fail</a></span>`,
			shortSHA,
		)
	}
	if state == "-3" {
		state_badge = `<span class="tag is-dark is-medium">Queued</span>`
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
					<button class="button is-info" id="copyBtn">Copy</button>
				</div>
	    	</div>
		<form action="/repo/%s/build" method="POST">
		<!-- Website URL Input -->
		<!-- Submit Button -->
		<div class="field is-grouped">
			<div class="control">
				<button class="button is-primary" type="submit">Build</button>
			</div>
		</div>
		</form>
		<span class="tag is-dark is-medium">%s | %s by %s </span> %s
		<hr>	
        <pre>%s</pre>
		%s
    	</div>
    </div>
 </body>
</html>`, 
html.Header(), 
html.NavBar(user_is_logged(c)),
uplink,
AppConfig.GitServerAddress,
c.Param("repo"), c.Param("repo"),
shortSHA, commit.Message, commit.Author.Email, state_badge,
data,
html.CopyPasteButtonScript(),
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
				"dump_file error, repo: %s, file: %s, error: %s", 
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

	edit_link := ""
	
	if user_is_logged(c) {
		edit_link = fmt.Sprintf(`<a class="button is-primary" href="/repo/%s/file_edit/%s">edit</a>`,repo,c.Param("file"))
	}
	
	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  <div>
        <p class="title">%s | %s %s</p>
         <hr>
        %s
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(user_is_logged(c)),
	link, 
	file_short_name,
	edit_link,
	code,
	),
)

}

func edit_file (c *echo.Context)  error {

	repo := c.Param("repo")

	dname := utils.GitCacheRootDir() + "/" + repo

	file := c.Param("file")

	content, err := os.ReadFile(dname + "/" + file)

	if err != nil {
		return c.String(
			http.StatusNotFound, 
			fmt.Sprintf(
				"edit_file error, repo: %s, file: %s, error: %s", 
				repo, file, err,
			),
		)
    }

	code := html_utils.EscapeString(string(content))

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
		<form id="code-form" action="/repo/%s/file/%s" method="POST">
			<textarea name="code" id="hidden-textarea" style="display: none;"></textarea>
			<button id="submit-code-btn" class="button is-primary" type="submit">Save</button>
      <hr>
      <div class="field">
        <label class="label">Message</label>
          <div class="control">
            <input name="message" class="input" type="text" placeholder="bug fix">
          </div>
          <p class="help">Commit message</p>
      </div>
		</form>
		<hr>
  	    <div id="notification-box" style="display: none; margin-top: 15px; padding: 12px; border-radius: 4px; font-size: 14px; transition: all 0.3s ease;"></div>
	  </div>
	</div>
	<div class="container">		
		<div id="editor" style="height: 400px;">%s</div>
	</div>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/ace/1.44.0/ace.js"></script>
	<script>
		var editor = ace.edit("editor");
		//editor.setTheme("ace/theme/monokai");
		//editor.session.setMode("ace/mode/yaml");
    	editor.session.setNewLineMode("unix");
		const codeForm = document.getElementById("code-form");
    	const hiddenTextarea = document.getElementById("hidden-textarea");
    	const submitBtn = document.getElementById("submit-code-btn");
		codeForm.addEventListener("submit", function(event) {
		const editorCode = editor.getValue().trim();
		hiddenTextarea.value = editorCode;
        });
	</script>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(user_is_logged(c)),
	link, 
	file_short_name, 
	c.Param("repo"),
	c.Param("file"),
	code,
	),
)

}

func change_file(c *echo.Context) error {

	if ! user_is_logged(c) {
		return c.Redirect(http.StatusMovedPermanently, "/login") 
	}

	u := new(ChangeFilePayload)

	// Функция Bind автоматически распарсит форму и заполнит структуру
	if err := c.Bind(u); err != nil {
		return c.String(http.StatusBadRequest, "Wrong input data")
	}

  dname := utils.GitCloneCacheRootDir() + "/" + c.Param("repo")

  // Удаляет директорию рекурсивно. Если её нет — ошибки не будет.
	err := os.RemoveAll(dname)
	if err != nil {
		fmt.Printf("change_file: can't remove directory %v\n", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error removing  directory")
	}

  fmt.Println("change_file: remove directory: %s", dname)

  err = os.MkdirAll(dname, 0755)

  if err != nil {
    log.Printf("change_file: error creating directory %s: %s", dname, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating directory")
  }

  log.Printf("change_file: creating git clone chache dir: %s OK", dname)

	RepoURL := fmt.Sprintf("http://localhost:8080/%s",c.Param("repo"))

	fmt.Printf("change_file: RepoURL: %s\n", RepoURL)

	fmt.Println("change_file: cloning repository ...")

	repo, err := go_git.PlainClone(dname, &go_git.CloneOptions{
		URL: RepoURL,
	})

	if err != nil {
		log.Printf("change_file: Failed to clone repo: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to clone repo")
	}

	filePath := c.Param("file")

  fullPath := filepath.Join(dname, filePath)

  output := strings.ReplaceAll(u.Code, "\r\n", "\n")

	err = os.WriteFile(fullPath,[]byte(output),0644)

	if err != nil {
		log.Printf("change_file: failed writing to file: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed writing to file")
	}

	worktree, err := repo.Worktree()

	if err != nil {
		log.Printf("change_file: failed to get worktree: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get worktree")
	}

	_, err = worktree.Add(filePath)

	if err != nil {
		log.Printf("change_file: failed to add file: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add file")

	}

	fmt.Printf("change_file: staged file: %s\n", filePath)

	
	commitMessage := "feat: add or update file via dsci web"

  if u.Message != "" {
    commitMessage = u.Message
  }

	commitHash, err := worktree.Commit(commitMessage, &go_git.CommitOptions{
		Author: &object.Signature{
			Name:  "DSCI Web",
			Email: "dsci@sparrowhub.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Printf("change_file: failed to commit: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit")
	}

	fmt.Printf("change_file: committed successfully. Hash: %s\n", commitHash)

	auth := &go_git_http.BasicAuth{
		Username: AppConfig.GitAuthUser,           
		Password: AppConfig.GitAuthPassword, 
	}

	err = repo.Push(&go_git.PushOptions{
		RemoteName: "origin",
		ClientOptions: []go_git_client.Option{
			go_git_client.WithHTTPAuth(auth),
		},
	})

	if err != nil {
		log.Printf("Failed pushing to git repository: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed pushing to bare repository")
	}

	fmt.Println("Successfully committed and pushed file to bare repository!")

	parts := strings.Split(c.Param("file"), "/")

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
			c.Param("repo"),
			c.Param("repo"),
		)
	} else {
		link = fmt.Sprintf(
			`<a href="/repo/%s">%s</a> | <a href="/repo/%s/dir/%s">%s/</a>`,
			c.Param("repo"),
			c.Param("repo"),
			c.Param("repo"),
			localdir,
			localdir,
		)
	}

	//edit_link := fmt.Sprintf(`<a class="button is-primary" href="/repo/%s/file_edit/%s">edit</a>`,c.Param("repo"),c.Param("file"))

	shortSHA := commitHash.String()[:7]

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  <div>
        <p class="title">%s | %s </p>
         <hr>
        Successfully committed and pushed: %s
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(user_is_logged(c)),
	link, 
	file_short_name,
	shortSHA,
	),
)
}


func manual_build(c *echo.Context) error {

	gitRoot, _ := filepath.Abs(repoRoot)

	repo_dir := gitRoot + "/" + c.Param("repo")

	repo, err := go_git.PlainOpen(repo_dir)

	if err != nil {
		log.Fatalf("manual_build: Failed to open repository: %v", err)
	}

	ref, _ := repo.Head()

	// 3. Parse the string SHA into a plumbing.Hash object
	hash := ref.Hash()

	// 4. Retrieve the commit object from the repository
	commit, err := repo.CommitObject(hash)
	if err != nil {
		log.Fatalf("manual_build: Failed to find commit %s: %v", "HEAD", err)
	}

	shortSHA := commit.Hash.String()[:7]

	msg := fmt.Sprintf(
		"%s by %s  - manual run",
		commit.Message,
		commit.Author.Email,
	)

	job_description := fmt.Sprintf("%s: %s | %s", c.Param("repo"), shortSHA, msg)

	// Strip the trailing .git extension to get the clean repository path identifier
	repoName := strings.TrimSuffix(c.Param("repo"), ".git")

    // JobQueue(app_cfg types.AppConfig, job_id string, msg string, repo string, ref, sha string,description string)
	job.JobQueue(
	    AppConfig,
	    commit.Hash.String(),
	    msg,
	    repoName,
	    string(ref.Name()),
	    shortSHA,
	    job_description,
    )

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  <div>
        <p class="title"><a href="/repo/%s">%s</a></p>
         <hr>
        job quedued: %s
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(user_is_logged(c)),
	c.Param("repo"),
	c.Param("repo"),
	shortSHA, 
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

func login_form(c *echo.Context) error {
	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  	<div>
			<p class="title">DSCI Login</p>
    	</div>
		<form id="login-form" action="/login" method="POST">
      		<div class="field">
        		<label class="label">Login</label>
          			<div class="control">
            			<input name="login" class="input" type="text" placeholder="username">
          			</div>
		          <p class="help">Login/Username</p>
      		</div>
      		<div class="field">
        		<label class="label">Password</label>
          			<div class="control">
            			<input name="password" class="input" type="password" placeholder="password">
          			</div>
		          <p class="help">Password/Token</p>
      		</div>
			<button id="submit-btn" class="button is-primary" type="submit">Submit</button>
		</form>
	</div>
 </body>
</html>`, 
html.Header(), 
html.NavBar(user_is_logged(c)),
))
}


func create_session ( c *echo.Context ) error {

	user := new(types.Session)

	if err := c.Bind(user); err != nil {
		return c.String(http.StatusBadRequest, "Wrong input data")
	}

	//log.Printf("create_session: %s %s", user.Login, user.Password)

	if ! (user.Login == AppConfig.GitAuthUser && user.Password == AppConfig.GitAuthPassword) {
		return c.HTML(
			http.StatusOK,
			fmt.Sprintf(
				`%s %s
		<div class="container">
		    <div>
			  <p class="title">	
			    <div class="help is-danger">Bad credentials</div>
			  </p>
      		</div>
		</div>
	 </body>
	</html>`, 
		html.Header(), 
		html.NavBar(user_is_logged(c)),
		),
		)		
	}

	dname := utils.SessionCacheRootDir()

	err := os.MkdirAll(dname, 0755)

	if err != nil {
	log.Printf("create_session: error creating directory %s: %s", dname, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating directory")
	}

	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var sb strings.Builder
	length := 10
	sb.Grow(length)
	for i := 0; i < length; i++ {
		// rand.IntN picks a random index from the charset
		sb.WriteByte(charset[rand.IntN(len(charset))])
	}	
	cookie := new(http.Cookie)
	session_id := sb.String()

	// Configure mandatory and security attributes
	cookie.Name = "user_session"
	cookie.Value = session_id
	cookie.Path = "/" // Accessible across the entire domain
	cookie.Expires = time.Now().Add(48 * time.Hour) // Valid for 2 days

	// Security Configurations (Highly Recommended)
	cookie.HttpOnly = true  // Prevents JavaScript/XSS attacks from reading the cookie
	//cookie.Secure = true    // Forces cookie transport only over HTTPS
	cookie.SameSite = http.SameSiteLaxMode // Protection against CSRF attacks

	// Send the cookie to the browser using Echo's Context
	c.SetCookie(cookie)  

	// Create the file (or truncate it if it already exists)

	path := dname + "/" + session_id + ".json"
	file, err := os.Create(path)
	if err != nil {
		log.Printf("create_session: error creating file: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating file")
	}
	defer file.Close()

	// Create a JSON encoder and write directly to the file
	encoder := json.NewEncoder(file)

	// Optional: Make the JSON file human-readable (pretty-printed)
	encoder.SetIndent("", "    ") 

	user.Password = "<censored>"

	err = encoder.Encode(user)

	if err != nil {
		log.Printf("create_session: error encoding session into json: %s", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error encoding session into json")
	}

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	    <div>
        <p class="title">User session</p>
         <hr>
			  Successfully logged in
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(true),
	),
	)	
}

func drop_session ( c *echo.Context ) error {

	cookie, err := c.Cookie("user_session")
	
	if err == nil {

		session := cookie.Value

		if session != "" {

			cookie := new(http.Cookie)
			cookie.Name = "user_session"
			cookie.Value = session
			cookie.Path = "/" 
			cookie.MaxAge = -1
			cookie.HttpOnly = true
			c.SetCookie(cookie)  
		
			path := fmt.Sprintf("%s/%s.json",utils.SessionCacheRootDir(),session)

			_, err = os.Stat(path);
				
			if err == nil {
				fmt.Printf("drop_session: delete session file: %s\n", path)
				err := os.Remove(path)
				if err != nil {
					fmt.Printf("drop_session: can't remove session file %v\n", err)
				} else {
					fmt.Printf("drop_session: delete session file OK\n")
				}			
			}
		
		}
	
	}

	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	    <div>
        <p class="title">User session</p>
        <hr>
		Successfully logged out
      </div>
    </div>
 </body>
</html>`, 
	html.Header(), 
	html.NavBar(user_is_logged(c)),
	),
	)	
}

func user_is_logged ( c *echo.Context ) bool {

	cookie, err := c.Cookie("user_session")
	
	if err != nil {
		return false
	}

	session := cookie.Value

	if session == "" {
		return false
	}

	//fmt.Printf("user_is_logged: session: %s\n", session)

	path := fmt.Sprintf("%s/%s.json",utils.SessionCacheRootDir(),session)

	_, err = os.Stat(path);

	//fmt.Printf("user_is_logged: session file: %s\n", path)

	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("user_is_logged: session file: %s does not exist\n", path)
		return false
	}

	file, err := os.Open(path)

	if err != nil {
		fmt.Printf("user_is_logged: Error opening file: %v\n", err)
		return false
	}

	defer file.Close() // Ensure the file is closed later

	// 3. Create an instance of your struct
	var user types.Session

	// 4. Decode the file content directly into the struct
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&user); err != nil {
		fmt.Printf("user_is_logged: Error decoding JSON: %v\n", err)
		return false
	}

	//fmt.Printf("user_is_logged:  true\n")

	return true
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
		"message":   "File succssfully created",
		"size_bytes": bytesWritten,
		"error" : "",
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
