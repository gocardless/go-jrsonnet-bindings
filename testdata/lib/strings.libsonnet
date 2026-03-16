{
  upper(s):: std.asciiUpper(s),
  repeat(s, n):: std.join("", std.makeArray(n, function(i) s)),
}
