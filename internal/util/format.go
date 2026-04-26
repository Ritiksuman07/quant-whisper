package util

import "fmt"

func FormatBytes(bytes uint64) string {
    const (
        kb = 1024
        mb = 1024 * kb
        gb = 1024 * mb
    )
    switch {
    case bytes >= gb:
        return fmt.Sprintf("%.1fGB", float64(bytes)/float64(gb))
    case bytes >= mb:
        return fmt.Sprintf("%.1fMB", float64(bytes)/float64(mb))
    case bytes >= kb:
        return fmt.Sprintf("%.1fKB", float64(bytes)/float64(kb))
    default:
        return fmt.Sprintf("%dB", bytes)
    }
}

func FormatPercent(value float64) string {
    return fmt.Sprintf("%.1f%%", value)
}
