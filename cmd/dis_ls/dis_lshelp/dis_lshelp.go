// Package lshelp provides common help for list commands.
package dis_lshelp

import (
	"strings"
)

// Help describes the common help for all the list commands
// Warning! "|" will be replaced by backticks below
var Help = strings.ReplaceAll(`
Any of the filtering options can be applied to this command.

There are several related list commands

  * |dis_ls| to list names of distributed objects

|dis_ls| are designed to be human-readable.

Note that |dis_ls| recurse by default - use |--max-depth 1| to stop the recursion.

Listing a nonexistent directory will produce an error except for
remotes which can't have empty directories (e.g. s3, swift, or gcs -
the bucket-based remotes).
`, "|", "`")
