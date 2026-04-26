package process

import "time"

type Entry struct {
    PID        int32
    Name       string
    Cmdline    []string
    Port       uint32
    Protocol   string
    Status     string
    CPUPercent float64
    MemRSS     uint64
    StartTime  time.Time
    User       string
    Exe        string
    Cwd        string
    PPID       int32
    ParentName string
}
