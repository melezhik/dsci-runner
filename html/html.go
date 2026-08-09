package html

import (
	"embed"
	"io/fs"
	"log"
)

// html helpers

//go:embed builds.html
var staticFiles embed.FS

func Header () (string ) {
	return `
<html data-theme="dark">
  <head>
    <meta charset="utf-8">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bulma@1.0.4/css/bulma.min.css">
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.15.0/dist/katex.min.css">
    <title>DSCI Jobs</title>
  </head>
  <body>
  `
}

func NavBar ( activer string ) (string) {
return `
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
        Home
      </a>

      <a href="https://github.com/melezhik/DSCI" class="navbar-item">
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
          <a href="https://discord.gg/KSMRTZ9F" class="navbar-item is-selected">
            Dsicord
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

    <div class="navbar-end">
      <div class="navbar-item">
        <div class="buttons">
          <a href="/builds" class="button is-primary">
            <strong>Pipelines</strong>
          </a>
        </div>
      </div>
    </div>
  </div>
</nav>
</div>
`
}


func LiveBuilds () (string) {

	content, err := fs.ReadFile(staticFiles, "builds.html")

	if err != nil {
			log.Fatalf("startJobDispatcher: error reading common/sparrowfile: %s", err)
	}

	return string(content)

}