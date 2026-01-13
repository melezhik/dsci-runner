use Cro::HTTP::Router;
use Cro::HTTP::Server;
use Cro::WebApp::Template;

my $application = route {


  post -> 'hook', {
    request-body -> %json {

      say "hit for {%json<after>}";

      my %trigger = %(
        description => "ci: sha={%json<after>}",
        #cwd => "/path/to/working/directory",
        sparrowdo => %(
        #  localhost => True,
        #  no_sudo   => True,
        #  conf      => "/path/to/file.conf"
          tags => "sha=tags/{%json<after>},scm={%json<repository><ssh_url>}",
        )
      );
      my $key = "{%json<after>}.{now.Int()}";
      mkdir "{%*ENV<HOME>}/.sparky/projects/dsci/.triggers/";
      "{%*ENV<HOME>}/.sparky/projects/dsci/.triggers/{$key}".IO.spurt(%trigger.raku);
      content 'text/plain', $key; 
    }
  }

}

my Cro::Service $service = Cro::HTTP::Server.new:
    :host(%*ENV<DSCI_HOST> || "127.0.0.1"), :port(%*ENV<DSCI_PORT> || 3333), :$application;

$service.start;

react whenever signal(SIGINT) {
    $service.stop;
    exit;
}

