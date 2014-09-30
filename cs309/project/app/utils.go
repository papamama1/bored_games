package app

import (
	"os/exec"
	"strings"
)

func GenerateUUID() string {
	out, err := exec.Command("uuidgen").Output()
	if err != nil {
		panic(err)
	}
	return strings.Trim(string(out), "\n")
}
