package main

import (
	"fmt"

	"github.com/LingByte/lingoroutine/version"
)

func main() {
	fmt.Println("LingoRoutine Framework")
	fmt.Println("======================")
	fmt.Println("Version:", version.GetVersion())
	fmt.Println("Version Info:", version.GetVersionInfo())
	fmt.Println("Git Commit:", version.GetGitCommit())
	fmt.Println("Build Time:", version.GetBuildTime())
	fmt.Println("Go Version:", version.GetGoVersion())
}
