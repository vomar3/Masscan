package scanner

import (
	"os/exec"
	"strconv"
)

func RunMasscan(masscanPath string, target string, ports string, rate int, output string) error {
	cmd := exec.Command(
		"wsl",
		masscanPath,
		target,
		"-p"+ports,
		"--rate",
		strconv.Itoa(rate),
		"-oJ",
		output,
	)

	return cmd.Run()
}
