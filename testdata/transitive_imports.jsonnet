local utils = import 'utils.libsonnet';
{
  shouted: utils.shout("hello"),
  repeated: utils.emphasize("ab", 3),
}
