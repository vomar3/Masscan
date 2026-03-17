package scanner

import (
	"context"
	"os"
	"os/exec"
	"strconv"
)

func RunMasscan(ctx context.Context, masscanPath string, target string, ports string, rate int, output string) error {
	cmd := exec.CommandContext(
		ctx,
		"wsl",
		masscanPath,
		target,
		"-p"+ports,
		"--rate",
		strconv.Itoa(rate),
		"-oJ",
		output,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	return nil
}
