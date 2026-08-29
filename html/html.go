package html

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"log"
)

// html helpers

//go:embed builds.html
var staticFiles embed.FS

func Header() string {
	return `
<html data-theme="dark">
  <head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.15.0/dist/katex.min.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/styles/default.min.css">
    <script src="https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.11.1/highlight.min.js"></script>
    <script>hljs.highlightAll();</script>
    <script>
      document.addEventListener('DOMContentLoaded', () => {
      // Get all "navbar-burger" elements
      const $navbarBurgers = Array.prototype.slice.call(document.querySelectorAll('.navbar-burger'), 0);
  
      // Add a click event on each of them
      $navbarBurgers.forEach( el => {
        el.addEventListener('click', () => {
          // Get the target from the "data-target" attribute
          const target = el.dataset.target;
          const $target = document.getElementById(target);
  
          // Toggle the "is-active" class on both the "navbar-burger" and the "navbar-menu"
          el.classList.toggle('is-active');
          $target.classList.toggle('is-active');
        });
      });
    });
    </script>
		<style type="text/css" media="screen">
				body {
					overflow: hidden;
				}
				#editor {
					margin: 0;
					position: absolute;
					top: 0;
					bottom: 0;
					left: 0;
					right: 0;
				}
		</style>
    <title>DSCI</title>
  </head>
  <body>
  `
}

func NavBar(logged bool) string {

	new_repo_ref := ""

	login_or_logout_ref := `
    <a href="/login" class="navbar-item">
        Login
    </a>`

	if logged == true {
		login_or_logout_ref = `
      <a href="/logout" class="navbar-item">
        Logout
      </a>`
		new_repo_ref = `
      <a href="/repo/create" class="navbar-item">
        New Repo
      </a>`
	}
	return fmt.Sprintf(`
<div class="container">
<nav class="navbar" role="navigation" aria-label="main navigation">
  <div class="navbar-brand">
    <a role="button" class="navbar-burger" aria-label="menu" aria-expanded="false" data-target="navbarBasicExample">
      <span aria-hidden="true"></span>
      <span aria-hidden="true"></span>
      <span aria-hidden="true"></span>
      <span aria-hidden="true"></span>
    </a>
  </div>

  <div id="navbarBasicExample" class="navbar-menu">
    <div class="navbar-start">
      <a href="/" class="navbar-item">
        Repos
      </a>
      <a href="/builds" class="navbar-item">
        Builds
      </a>
      %s
      %s
      <a href="http://deadsimpleci.sparrowhub.io" class="navbar-item">
        Documentation
      </a>
      <div class="navbar-item has-dropdown is-hoverable">
        <a class="navbar-link">
          More
        </a>
        <div class="navbar-dropdown">
          <a href="https://github.com/melezhik/DSCI" class="navbar-item">
            GitHub
          </a>
          <a href="https://discord.gg/KSMRTZ9F" class="navbar-item">
            Discord
          </a>
          <a href="https://discord.gg/KSMRTZ9F" class="navbar-item">
            Contact
          </a>
          <hr class="navbar-divider">
          <a href="https://github.com/melezhik/DSCI/issues" class="navbar-item">
            Report an issue
          </a>
        </div>
      </div>
    </div>  
  </div>
</nav>
</div>
`, new_repo_ref, login_or_logout_ref)
}

func LiveBuilds() string {

	content, err := fs.ReadFile(staticFiles, "builds.html")

	if err != nil {
		log.Fatalf("LiveBuilds: error reading common/sparrowfile: %s", err)
	}

	return string(content)

}

func CodeToHtml(code string) string {

	safe := html.EscapeString(code)

	return fmt.Sprintf("<pre><code>%s</code></pre>", safe)
}

func CopyPasteButtonScript() string {

	return `<script>	
  document.getElementById('copyBtn').addEventListener('click', function() {
    var input = document.getElementById('copyInput');
    
    // Select the text field
    input.select();
    input.setSelectionRange(0, 99999); // For mobile devices
    
    try {
      // Legacy command works on non-TLS/HTTP sites
      var successful = document.execCommand('copy');
      if (successful) {
        this.textContent = 'Copied!';
        setTimeout(() => { this.textContent = 'Copy'; }, 2000);
      } else {
        alert('Failed to copy');
      }
    } catch (err) {
      alert('Browser not supported');
    }
  });
</script>`
}
