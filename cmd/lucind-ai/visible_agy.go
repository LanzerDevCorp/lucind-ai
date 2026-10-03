package main

import (
	"os"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

// visibleOptOut reports whether the owner turned the visible sessions off
// (LUCIND_HERDR_VISIBLE=off): headless agy everywhere, as before.
func visibleOptOut() bool { return os.Getenv("LUCIND_HERDR_VISIBLE") == "off" }

// agyExecutor is the executor for packets that say `executor: agy`. Inside herdr (HERDR_ENV=1) the
// lane runs as a visible interactive agy session in its own pane, so the owner always sees the
// agent work; anywhere else (CI, no herdr) it stays the headless `agy --print` process.
func agyExecutor() executor.Executor {
	if os.Getenv("HERDR_ENV") == "1" && !visibleOptOut() {
		return executor.HerdrAgy{Interactive: true}
	}
	return executor.Agy{}
}

// herdrAgyExecutor is the explicit herdr executor: interactive unless the owner opted out.
func herdrAgyExecutor() executor.Executor {
	return executor.HerdrAgy{Interactive: !visibleOptOut()}
}
