package html

// html helpers

func NavBar ( activer string ) (string) {
return `
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

      <a class="navbar-item">
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
`

}