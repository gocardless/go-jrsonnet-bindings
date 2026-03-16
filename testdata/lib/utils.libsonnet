local strings = import 'strings.libsonnet';
{
  shout(name):: strings.upper(name) + "!",
  emphasize(name, n):: strings.repeat(name, n),
}
