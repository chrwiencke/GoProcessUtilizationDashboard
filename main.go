package main

import (
    "github.com/dustin/go-humanize"
    "github.com/gin-gonic/gin"
    "github.com/rs/zerolog/log"
    "github.com/shirou/gopsutil/cpu"
    "github.com/shirou/gopsutil/host"
    "github.com/shirou/gopsutil/mem"
    "github.com/shirou/gopsutil/disk"
    "github.com/shirou/gopsutil/net"
    "net/http"
    "time"
    "bufio"
    "os"
    "path/filepath"
    "strings"
    "runtime"
    "sort"
)

type Metrics struct {
    MemoryUsage    float64 `json:"memoryUsage"`
    CPUUsage       float64 `json:"cpuUsage"`
    FormattedTotal string  `json:"formattedTotal"`
    FormattedFree  string  `json:"formattedFree"`
    UptimeHours    uint64  `json:"uptimeHours"`
    UptimeMinutes  uint64  `json:"uptimeMinutes"`
    UptimeSeconds  uint64  `json:"uptimeSeconds"`
    Hostname       string  `json:"hostname"`
    Platform       string  `json:"platform"`
    OS             string  `json:"os"`
    KernelArch     string  `json:"kernelArch"`
    CPUCores       []float64 `json:"cpuCores"`
    DiskUsage      map[string]DiskInfo `json:"diskUsage"`
    NetworkIO      map[string]NetInfo  `json:"networkIO"`
}

type DiskInfo struct {
    Path        string  `json:"path"`
    Total       string  `json:"total"`
    Used        string  `json:"used"`
    UsedPercent float64 `json:"usedPercent"`
}

type NetInfo struct {
    BytesSent   string `json:"bytesSent"`
    BytesRecv   string `json:"bytesRecv"`
    PacketsSent uint64 `json:"packetsSent"`
    PacketsRecv uint64 `json:"packetsRecv"`
}

type LogEntry struct {
    Timestamp string `json:"timestamp"`
    Level     string `json:"level"`
    Message   string `json:"message"`
}

var systemLogPaths = map[string][]string{
    "darwin": {
        "/var/log/system.log",
        "/var/log/install.log",
        "/var/log/wifi.log",
    },
    "linux": {
        "/var/log/syslog",
        "/var/log/auth.log",
        "/var/log/kern.log",
        "/var/log/dmesg",
    },
    "windows": {
        "Application",
        "System",
        "Security",
    },
}

func getPriority(logLine string) string {
    lower := strings.ToLower(logLine)
    if strings.Contains(lower, "error") || strings.Contains(lower, "emergency") || 
       strings.Contains(lower, "alert") || strings.Contains(lower, "critical") {
        return "high"
    }
    if strings.Contains(lower, "warning") || strings.Contains(lower, "warn") {
        return "medium"
    }
    return "low"
}

func readSystemLogs() ([]LogEntry, error) {
    var logs []LogEntry
    osType := runtime.GOOS

    if osType == "windows" {
        return logs, nil
    }

    for _, logPath := range systemLogPaths[osType] {
        file, err := os.OpenFile(logPath, os.O_RDONLY, 0644)
        if err != nil {
            continue
        }
        defer file.Close()

        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            line := scanner.Text()
            priority := getPriority(line)
            parts := strings.SplitN(line, " ", 3)
            if len(parts) < 3 {
                continue
            }
            timestamp := parts[0] + " " + parts[1]
            message := parts[2]
            logs = append(logs, LogEntry{
                Timestamp: timestamp,
                Level:     priority,
                Message:   message,
            })
        }
    }
    return logs, nil
}

func getMetrics() Metrics {
    v, _ := mem.VirtualMemory()
    c, _ := cpu.Percent(time.Second, true)
    uptime, _ := host.Uptime()
    hostname, _ := host.Info()
    
    diskUsage := make(map[string]DiskInfo)
    partitions, _ := disk.Partitions(false)
    for _, partition := range partitions {
        usage, err := disk.Usage(partition.Mountpoint)
        if err == nil {
            diskUsage[partition.Mountpoint] = DiskInfo{
                Path:        partition.Mountpoint,
                Total:      humanize.Bytes(usage.Total),
                Used:       humanize.Bytes(usage.Used),
                UsedPercent: usage.UsedPercent,
            }
        }
    }

    netIO := make(map[string]NetInfo)
    interfaces, _ := net.Interfaces()
    for _, iface := range interfaces {
        stats, err := net.IOCounters(true)
        if err == nil {
            for _, s := range stats {
                if s.Name == iface.Name {
                    netIO[iface.Name] = NetInfo{
                        BytesSent:   humanize.Bytes(s.BytesSent),
                        BytesRecv:   humanize.Bytes(s.BytesRecv),
                        PacketsSent: s.PacketsSent,
                        PacketsRecv: s.PacketsRecv,
                    }
                }
            }
        }
    }

    return Metrics{
        MemoryUsage:    v.UsedPercent,
        CPUUsage:       c[0],
        FormattedTotal: humanize.Bytes(v.Total),
        FormattedFree:  humanize.Bytes(v.Free),
        UptimeHours:    uptime / 3600,
        UptimeMinutes:  (uptime % 3600) / 60,
        UptimeSeconds:  uptime % 60,
        Hostname:       hostname.Hostname,
        Platform:       hostname.Platform,
        OS:             hostname.OS,
        KernelArch:     hostname.KernelArch,
        CPUCores:       c,
        DiskUsage:      diskUsage,
        NetworkIO:      netIO,
    }
}

func readLogs(priority string) ([]LogEntry, error) {
    logPath := filepath.Join("logs", priority+".log")
    file, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDONLY, 0644)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var logs []LogEntry
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        parts := strings.SplitN(line, " | ", 3)
        if len(parts) == 3 {
            logs = append(logs, LogEntry{
                Timestamp: parts[0],
                Level:     parts[1],
                Message:   parts[2],
            })
        }
    }
    return logs, scanner.Err()
}

func main() {
    r := gin.Default()
    r.LoadHTMLGlob("templates/*")

    r.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "metrics.html", getMetrics())
    })

    r.GET("/metrics", func(c *gin.Context) {
        c.JSON(http.StatusOK, getMetrics())
    })

    r.GET("/logs", func(c *gin.Context) {
        c.HTML(http.StatusOK, "logs.html", nil)
    })

    r.GET("/logs/high", func(c *gin.Context) {
        c.HTML(http.StatusOK, "high-priority.html", nil)
    })

    r.GET("/logs/medium", func(c *gin.Context) {
        c.HTML(http.StatusOK, "medium-priority.html", nil)
    })

    r.GET("/logs/low", func(c *gin.Context) {
        c.HTML(http.StatusOK, "low-priority.html", nil)
    })

    r.GET("/api/logs/:priority", func(c *gin.Context) {
        priority := c.Param("priority")
        
        logs, err := readSystemLogs()
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        if priority != "all" {
            filteredLogs := make([]LogEntry, 0)
            for _, log := range logs {
                if log.Level == priority {
                    filteredLogs = append(filteredLogs, log)
                }
            }
            logs = filteredLogs
        }

        sort.Slice(logs, func(i, j int) bool {
            return logs[i].Timestamp > logs[j].Timestamp
        })

        if len(logs) > 1000 {
            logs = logs[len(logs)-1000:]
        }

        c.JSON(http.StatusOK, logs)
    })

    r.POST("/api/logs/:priority", func(c *gin.Context) {
        priority := c.Param("priority")
        var input struct {
            Message string `json:"message"`
        }
        if err := c.BindJSON(&input); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }

        logPath := filepath.Join("logs", priority+".log")
        file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
        defer file.Close()

        logEntry := time.Now().Format(time.RFC3339) + " | " + priority + " | " + input.Message + "\n"
        if _, err := file.WriteString(logEntry); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        c.JSON(http.StatusOK, gin.H{"status": "log written"})
    })

    log.Info().Msg("Server starting at http://localhost:8080")
    r.Run(":8080")
}
