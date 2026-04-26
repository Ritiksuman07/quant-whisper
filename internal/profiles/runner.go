package profiles

import (
    "os"
    "os/exec"
    "runtime"
    "time"
)

func Launch(p Profile) error {
    now := time.Now()
    p.LastLaunchedAt = &now

    for _, cmd := range p.Commands {
        if cmd.Cmd == "" {
            continue
        }
        execCmd := buildShellCommand(cmd.Cmd)
        if dir := ExpandHome(cmd.Dir); dir != "" {
            execCmd.Dir = dir
        }
        execCmd.Env = append(os.Environ(), cmd.Env...)
        if err := execCmd.Start(); err != nil {
            return err
        }
    }
    return nil
}

func buildShellCommand(command string) *exec.Cmd {
    if runtime.GOOS == "windows" {
        return exec.Command("cmd", "/C", command)
    }
    return exec.Command("sh", "-c", command)
}
