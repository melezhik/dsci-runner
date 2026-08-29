package main

import (
	"bufio"
	"bytes"
	"dsci_runner/git"
	"dsci_runner/job"
	"dsci_runner/types"
	"dsci_runner/utils"
	"embed"
	"errors"
	"flag"
	"fmt"
	go_git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pelletier/go-toml/v2"
	"io"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"context"
	"os/signal"
	"syscall"
)

// Git related constants
// TODO: move to ~/.dsci.toml

const (
	repoRoot = ".repositories"
	sshAddr  = ":2222"
)

//go:embed common
var staticFiles12 embed.FS

var AppConfig types.AppConfig

type ChangeFilePayload struct {
	Code    string `form:"code"`
	Message string `form:"message"`
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
	e1.POST("/repo/:repo/build", manual_build)
	e1.GET("/livebuilds", livebuilds)

	// e1.POST("/repo/create", create_repo)
	e1.GET("/repo/create", create_repo_form)

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
					job_id := shortSHA
					// JobQueue(app_cfg types.AppConfig, job_id string, msg string, repo string, ref, sha string,description string)
					job.JobQueue(
						AppConfig,
						job_id,
						msg,
						repoName,
						data[0].RefName,
						shortSHA,
						job_description,
					)
					job.UpdateCommitToJobIdState(shortSHA, job_id)
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

// Handlers - see hadlers_*.go files

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
