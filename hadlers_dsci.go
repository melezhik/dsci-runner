// DSCI CI related handlers
package main

import (

	"dsci_runner/job"
	"dsci_runner/utils"
	"dsci_runner/html"
	"fmt"
	"github.com/labstack/echo/v5"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"errors"

	_ "github.com/mattn/go-sqlite3"
	"dsci_runner/types"
	"io"
	"encoding/json"
	"github.com/robert-nix/ansihtml"
	"golang.org/x/net/websocket"
	"database/sql"
	"math/rand/v2"


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