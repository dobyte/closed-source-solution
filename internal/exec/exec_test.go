package exec_test

import (
	"fmt"
	"testing"

	"github.com/dobyte/closed-source-solution/internal/exec"
)

func TestExecGoEnv(t *testing.T) {
	envs, err := exec.GoEnv()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(envs)
}
