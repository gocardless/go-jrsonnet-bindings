// Both helper and utils are independent libs, but utils transitively imports strings.
// This tests that the VM correctly handles a file importing multiple libs
// where some of those libs share transitive dependencies.
local helper = import 'helper.libsonnet';
local utils = import 'utils.libsonnet';
local strings = import 'strings.libsonnet';
{
  greet: helper.greet("world"),
  shout: utils.shout("world"),
  direct_upper: strings.upper("test"),
}
