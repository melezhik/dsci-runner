my $exp = q[ ( .<ref> eq "KKK" && .<foo> > 10 ) || .<ff> < 3 or .<fff> ~~ m:i:g /OK $$/  ];


say "===";
say $exp;
say "===";
say "validate";

my @b;

for $exp.split(/\s [ "and" || "or" || "&&" || "||" ]  \s/)
.map({
  .subst("(","",:g)
  .subst(")","",:g)
  .subst(/^^ \s+/,"")
  .subst(/\s+ $$/,"")
  .subst('$$',"rx_end_of_line",:g)
  }) -> $i {
  say "parse [$i]";  
  if  $i !~~ /
      ^^ 
        ".<" 
          \w+ 
        ">" 
      \s+ 
      [ 
        ">" || ">=" || "<" || "<=" || "eq" || "ne" ||  "~~" || "!~~" 
      ]
      \s+
      [
        \" [ \w+ || \d ] \" || \d+ || "m:" <[i m g r s \:]> + \s+ \/ <-[ \{ \{ \$  ]> +  \/
      ] 
      $$
    / {
    push @b, $i
  }
}

say "==";
for @b -> $i {
  say "bogus chunk found: $i"
}
