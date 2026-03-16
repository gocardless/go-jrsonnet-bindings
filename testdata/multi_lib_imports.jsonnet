local helper = import 'helper.libsonnet';
local utils = import 'utils.libsonnet';
local math = import 'math.libsonnet';
{
  greeting: helper.greet("world"),
  shouted: utils.shout("world"),
  doubled: math.double(21),
}
