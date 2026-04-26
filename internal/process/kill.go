package process

import (
    "context"
    "os"
    "os/exec"
    "runtime"
    "syscall"
    "time"
)

func KillProcess(ctx context.Context, pid int32) error {
    proc, err := os.FindProcess(int(pid))
    if err != nil {
        return err
    }

    if runtime.GOOS == "windows" {
        return proc.Kill()
    }

    if err := proc.Signal(syscall.SIGTERM); err != nil {
        return err
    }

    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(3 * time.Second):
    }

    _ = proc.Signal(syscall.SIGKILL)
    return nil
}

func RestartProcess(ctx context.Context, entry Entry) error {
    if len(entry.Cmdline) == 0 {
        return ErrCommandUnknown
    }

    if err := KillProcess(ctx, entry.PID); err != nil {
        return err
    }

    cmd := exec.Command(entry.Cmdline[0], entry.Cmdline[1:]...)
    if entry.Cwd != "" {
        cmd.Dir = entry.Cwd
    }
    cmd.Env = os.Environ()
    return cmd.Start()
}
