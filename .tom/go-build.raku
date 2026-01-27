my $path = [
  "main.go",
];

task-run "build", "go-build", %(
  :$path,
);
