package process

import (
    "context"
    "errors"
    "sort"
    "strings"
    "syscall"
    "time"

    "github.com/shirou/gopsutil/v3/net"
    gopsproc "github.com/shirou/gopsutil/v3/process"
)

type Scanner struct {
    interval time.Duration
}

func NewScanner(interval time.Duration) *Scanner {
    if interval <= 0 {
        interval = 2 * time.Second
    }
    return &Scanner{interval: interval}
}

func (s *Scanner) Start(ctx context.Context) <-chan []Entry {
    ch := make(chan []Entry)
    go func() {
        ticker := time.NewTicker(s.interval)
        defer ticker.Stop()
        defer close(ch)

        for {
            entries, _ := ScanOnce()
            ch <- entries

            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
            }
        }
    }()
    return ch
}

func ScanOnce() ([]Entry, error) {
    conns, err := net.Connections("inet")
    if err != nil {
        return nil, err
    }

    type procInfo struct {
        name     string
        cmdline  []string
        cpu      float64
        mem      uint64
        user     string
        exe      string
        cwd      string
        ppid     int32
        start    time.Time
        parent   string
    }

    procCache := map[int32]*procInfo{}
    entries := make([]Entry, 0, len(conns))

    for _, c := range conns {
        if c.Pid == 0 {
            continue
        }
        if c.Status == "" && c.Laddr.Port == 0 {
            continue
        }
        if c.Laddr.Port == 0 {
            continue
        }

        pinfo, ok := procCache[c.Pid]
        if !ok {
            pinfo = &procInfo{}
            p, perr := gopsproc.NewProcess(c.Pid)
            if perr == nil {
                if name, err := p.Name(); err == nil {
                    pinfo.name = name
                }
                if cmd, err := p.CmdlineSlice(); err == nil {
                    pinfo.cmdline = cmd
                }
                if cpu, err := p.CPUPercent(); err == nil {
                    pinfo.cpu = cpu
                }
                if mem, err := p.MemoryInfo(); err == nil && mem != nil {
                    pinfo.mem = mem.RSS
                }
                if user, err := p.Username(); err == nil {
                    pinfo.user = user
                }
                if exe, err := p.Exe(); err == nil {
                    pinfo.exe = exe
                }
                if cwd, err := p.Cwd(); err == nil {
                    pinfo.cwd = cwd
                }
                if ppid, err := p.Ppid(); err == nil {
                    pinfo.ppid = ppid
                }
                if ct, err := p.CreateTime(); err == nil {
                    pinfo.start = time.UnixMilli(ct)
                }
                if pinfo.ppid > 0 {
                    if parent, err := gopsproc.NewProcess(pinfo.ppid); err == nil {
                        if pname, err := parent.Name(); err == nil {
                            pinfo.parent = pname
                        }
                    }
                }
            }
            procCache[c.Pid] = pinfo
        }

        protocol := "UNKNOWN"
        switch c.Type {
        case syscall.SOCK_STREAM:
            protocol = "TCP"
        case syscall.SOCK_DGRAM:
            protocol = "UDP"
        }

        entries = append(entries, Entry{
            PID:        c.Pid,
            Name:       pinfo.name,
            Cmdline:    pinfo.cmdline,
            Port:       c.Laddr.Port,
            Protocol:   protocol,
            Status:     strings.ToUpper(c.Status),
            CPUPercent: pinfo.cpu,
            MemRSS:     pinfo.mem,
            StartTime:  pinfo.start,
            User:       pinfo.user,
            Exe:        pinfo.exe,
            Cwd:        pinfo.cwd,
            PPID:       pinfo.ppid,
            ParentName: pinfo.parent,
        })
    }

    sort.Slice(entries, func(i, j int) bool {
        if entries[i].Port == entries[j].Port {
            return entries[i].PID < entries[j].PID
        }
        return entries[i].Port < entries[j].Port
    })

    return entries, nil
}

var ErrCommandUnknown = errors.New("original command unknown")
