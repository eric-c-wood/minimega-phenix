// Copyright 2018-2021 National Technology & Engineering Solutions of Sandia, LLC (NTESS).
// Under the terms of Contract DE-NA0003525 with NTESS, the U.S. Government retains certain
// rights in this software.

package minicli_test

import (
	log "minilog"
)

func init() {
	// Setup up default logger to log to stdout at the debug level
	log.LevelFlag = log.DEBUG
	log.VerboseFlag = true

	log.Init()
}
