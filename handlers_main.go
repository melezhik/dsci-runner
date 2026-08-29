// Main handlers
package main

import (
	"dsci_runner/html"
	"dsci_runner/job"
	"dsci_runner/types"
	"dsci_runner/utils"
	"encoding/json"
	"errors"
	"fmt"
	go_git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/config"
	go_git_client "github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	go_git_http "github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/labstack/echo/v5"
	_ "github.com/mattn/go-sqlite3"
	html_utils "html"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ChangeFilePayload struct {
	Code    string `form:"code"`
	Message string `form:"message"`
}

type CreateRepoPayload struct {
	Repo    string `form:"repo"`
	Migrate string `form:"migrate"`
}

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

	job_id := job.CommitToJobId(shortSHA)

	state := job.JobState("dsci", job_id)

	state_badge := `<span class="tag is-white is-medium">Unknown</span>`

	if state == "0" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-warning is-medium"><a href="/report/ui/dsci/%s">Running</a></span>`,
			job_id,
		)
	}
	if state == "1" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-success is-medium"><a href="/report/ui/dsci/%s">Pass</a></span>`,
			job_id,
		)
	}
	if state == "-1" {
		state_badge = fmt.Sprintf(
			`<span class="tag is-danger" is-medium><a href="/report/ui/dsci/%s">Fail</a></span>`,
			job_id,
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

func dump_file(c *echo.Context) error {

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
		edit_link = fmt.Sprintf(`<a class="button is-primary" href="/repo/%s/file_edit/%s">edit</a>`, repo, c.Param("file"))
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

func edit_file(c *echo.Context) error {

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

	if !user_is_logged(c) {
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

	fmt.Printf("change_file: remove directory: %s", dname)

	err = os.MkdirAll(dname, 0755)

	if err != nil {
		log.Printf("change_file: error creating directory %s: %s", dname, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating directory")
	}

	log.Printf("change_file: creating git clone chache dir: %s OK", dname)

	RepoURL := fmt.Sprintf("http://localhost:8080/%s", c.Param("repo"))

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

	err = os.WriteFile(fullPath, []byte(output), 0644)

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
		log.Printf("change_file: failed pushing to git repository: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed pushing to bare repository")
	}

	fmt.Println("change_file: Successfully committed and pushed file to bare repository!")

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

	job_id := fmt.Sprintf("%s.%d", shortSHA, time.Now().Unix())

	job_description := fmt.Sprintf("%s: %s | %s", c.Param("repo"), job_id, msg)

	// Strip the trailing .git extension to get the clean repository path identifier
	repoName := strings.TrimSuffix(c.Param("repo"), ".git")

	// JobQueue(app_cfg types.AppConfig, job_id string, msg string, repo string, ref, sha string,description string)
	job.JobQueue(
		AppConfig,
		job_id,
		msg,
		repoName,
		string(ref.Name()),
		shortSHA,
		job_description,
	)
	job.UpdateCommitToJobIdState(shortSHA, job_id)
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
			job_id,
		),
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

func create_session(c *echo.Context) error {

	user := new(types.Session)

	if err := c.Bind(user); err != nil {
		return c.String(http.StatusBadRequest, "Wrong input data")
	}

	//log.Printf("create_session: %s %s", user.Login, user.Password)

	if !(user.Login == AppConfig.GitAuthUser && user.Password == AppConfig.GitAuthPassword) {
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
	cookie.Path = "/"                               // Accessible across the entire domain
	cookie.Expires = time.Now().Add(48 * time.Hour) // Valid for 2 days

	// Security Configurations (Highly Recommended)
	cookie.HttpOnly = true // Prevents JavaScript/XSS attacks from reading the cookie
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

func drop_session(c *echo.Context) error {

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

			path := fmt.Sprintf("%s/%s.json", utils.SessionCacheRootDir(), session)

			_, err = os.Stat(path)

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

func user_is_logged(c *echo.Context) bool {

	cookie, err := c.Cookie("user_session")

	if err != nil {
		return false
	}

	session := cookie.Value

	if session == "" {
		return false
	}

	//fmt.Printf("user_is_logged: session: %s\n", session)

	path := fmt.Sprintf("%s/%s.json", utils.SessionCacheRootDir(), session)

	_, err = os.Stat(path)

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


func create_repo_form (c *echo.Context) error {
	return c.HTML(
		http.StatusOK,
		fmt.Sprintf(
			`%s %s
    <div class="container">
	  	<div>
			<p class="title">New Git Repo</p>
    	</div>
		<form id="login-form" action="/repo/create" method="POST">
      		<div class="field">
        		<label class="label">Repo/Url</label>
          			<div class="control">
            			<input name="repo" class="input" type="text" placeholder="repo name">
          			</div>
		          <p class="help">Repo/Git Url</p>
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

func create_repo(c *echo.Context) error {

	if !user_is_logged(c) {
		return c.Redirect(http.StatusMovedPermanently, "/login")
	}

	r := new(CreateRepoPayload)

	// Функция Bind автоматически распарсит форму и заполнит структуру
	if err := c.Bind(r); err != nil {
		return c.String(http.StatusBadRequest, "Wrong input data")
	}

	repoName := r.Repo

	if ! strings.HasSuffix(repoName, ".git") {
		repoName = fmt.Sprintf("%s.git",repoName)
	}

	log.Printf("create_repo: %s, migrate: %s",repoName,r.Migrate)

	dname, _ := filepath.Abs(repoRoot)

	cmd := exec.Command("git","init","--bare",repoName)

	if r.Migrate == "on" {
		cmd = exec.Command("git","clone","--bare",repoName)
	}
	cmd.Dir = dname

	log.Printf("create_repo: git init command: %s",cmd)

	output, err := cmd.CombinedOutput() // Run the command and wait for completion

	if err != nil {
		log.Printf("create_repo: git init failed with: %s\n", output)
		return echo.NewHTTPError(http.StatusInternalServerError, "git init failed")
	}

	log.Printf("create_repo: git init OK: %s", output)

	dname = utils.GitCloneCacheRootDir() + "/" + repoName

	err = os.RemoveAll(dname)
	if err != nil {
		fmt.Printf("create_repo: can't remove directory %v\n", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error removing  directory")
	}

	fmt.Printf("create_repo: remove directory: %s", dname)

	err = os.MkdirAll(dname, 0755)

	if err != nil {
		log.Printf("create_repo: error creating directory %s: %s", dname, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating directory")
	}

	log.Printf("create_repo: creating git clone chache dir: %s OK", dname)

	RepoURL := fmt.Sprintf("http://localhost:8080/%s", repoName)

	fmt.Printf("create_repo: RepoURL: %s\n", RepoURL)

	var repo *go_git.Repository 

	if r.Migrate == "" {

		fmt.Printf("create_repo: init empty local git repository: %s ...\n", dname)

		// Init blank git repo
		repo_, err := go_git.PlainInit(dname, false)
		if err != nil {
			log.Printf("create_repo: failed to init empty repo: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to init empty repo")
		}
		// Create origin remote pointing to the empty target
		remote_url := fmt.Sprintf("http://127.0.0.1:8080/%s",repoName)
		_, err = repo_.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{remote_url},
		})
		if err != nil {
			log.Printf("create_repo: Failed to create remote for local repo: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create remote for local repo")
		}
		repo = repo_
	} else {

		fmt.Println("create_repo: cloning repository ...")

		repo_, err := go_git.PlainClone(dname, &go_git.CloneOptions{
			URL: RepoURL,
		})
	
		if err != nil {
			log.Printf("create_repo: Failed to clone repo: %v", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to clone repo")
		}
		repo = repo_
	}


	dname = filepath.Join(dname, ".dsci")

	err = os.MkdirAll(dname, 0755)

	if err != nil {
		log.Printf("create_repo: error creating .dsci directory %s: %s", dname, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error creating .dsci directory")
	}

	filePath := "jobs.yaml"

	fullPath := filepath.Join(dname, filePath)

	jobs_yaml := fmt.Sprintf("jobs:\n -\n  id: test\n  path: .\n")

	err = os.WriteFile(fullPath, []byte(jobs_yaml), 0644)

	if err != nil {
		log.Printf("create_repo: failed writing to file: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed writing to file")
	}

	filePath = "task.bash"

	fullPath = filepath.Join(dname, filePath)

	task_bash := `echo "hello DSCI"`

	err = os.WriteFile(fullPath, []byte(task_bash), 0644)

	if err != nil {
		log.Printf("create_repo: failed writing to file: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed writing to file")
	}

	worktree, err := repo.Worktree()

	if err != nil {
		log.Printf("create_repo: failed to get worktree: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get worktree")
	}

	_, err = worktree.Add(".dsci")

	if err != nil {
		log.Printf("create_repo: failed to add dir: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to add dir")

	}

	fmt.Printf("create_repo: staged file: %s\n", filePath)

	commitMessage := "feat: add or update file via dsci web"

	commitHash, err := worktree.Commit(commitMessage, &go_git.CommitOptions{
		Author: &object.Signature{
			Name:  "DSCI Web",
			Email: "dsci@sparrowhub.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Printf("create_repo: failed to commit: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit")
	}

	fmt.Printf("create_repo: committed successfully. Hash: %s\n", commitHash)

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
		log.Printf("create_repo: failed pushing to git repository: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed pushing to bare repository")
	}

	fmt.Println("create_repo: Successfully committed and pushed file to bare repository!")

	return c.Redirect(http.StatusMovedPermanently, "/repo/" + repoName)

}
