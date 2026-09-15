package probe

import (
	"bufio"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/dbohry/swtop/internal/model"
)

const Script = `
echo '@@CPU'
cat /proc/stat 2>/dev/null
echo '@@MEM'
cat /proc/meminfo 2>/dev/null
echo '@@LOAD'
cat /proc/loadavg 2>/dev/null
echo '@@UPTIME'
cat /proc/uptime 2>/dev/null
echo '@@NET'
cat /proc/net/dev 2>/dev/null
echo '@@DISK'
df -kP / 2>/dev/null | tail -n +2
echo '@@DOCKERSTATS'
docker stats --no-stream --no-trunc --format '{{json .}}' 2>/dev/null
echo '@@DOCKERPS'
docker ps --no-trunc --format '{{json .}}' 2>/dev/null
`

type cpuCounters struct {
	total uint64
	idle  uint64
}

type Sample struct {
	Timestamp time.Time
	CPUAll    cpuCounters
	CPUCores  []cpuCounters
	NetRxTot  uint64
	NetTxTot  uint64
}

type sections struct {
	cpu, mem, load, uptime, net, disk, dockerStats, dockerPS []string
}

func splitSections(output string) sections {
	var s sections
	markers := map[string]*[]string{
		"@@CPU":         &s.cpu,
		"@@MEM":         &s.mem,
		"@@LOAD":        &s.load,
		"@@UPTIME":      &s.uptime,
		"@@NET":         &s.net,
		"@@DISK":        &s.disk,
		"@@DOCKERSTATS": &s.dockerStats,
		"@@DOCKERPS":    &s.dockerPS,
	}

	var cur *[]string
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(nil, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if p, ok := markers[line]; ok {
			cur = p
			continue
		}
		if cur != nil {
			*cur = append(*cur, line)
		}
	}
	return s
}

func parseCPU(lines []string) (cpuCounters, []cpuCounters) {
	var all cpuCounters
	var cores []cpuCounters
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.HasPrefix(fields[0], "cpu") {
			continue
		}
		nums := make([]uint64, 0, len(fields)-1)
		for _, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				break
			}
			nums = append(nums, v)
		}
		if len(nums) < 4 {
			continue
		}
		var total uint64
		for _, n := range nums {
			total += n
		}
		idle := nums[3]
		if len(nums) > 4 {
			idle += nums[4]
		}
		c := cpuCounters{total: total, idle: idle}
		if fields[0] == "cpu" {
			all = c
		} else {
			cores = append(cores, c)
		}
	}
	return all, cores
}

func parseMem(lines []string) (totalKB, usedKB, swapTotalKB, swapUsedKB uint64) {
	vals := map[string]uint64{}
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		vals[key] = v
	}
	totalKB = vals["MemTotal"]
	avail, ok := vals["MemAvailable"]
	if !ok {
		avail = vals["MemFree"]
	}
	if totalKB > avail {
		usedKB = totalKB - avail
	}
	swapTotalKB = vals["SwapTotal"]
	swapFree := vals["SwapFree"]
	if swapTotalKB > swapFree {
		swapUsedKB = swapTotalKB - swapFree
	}
	return
}

func parseLoad(lines []string) (l1, l5, l15 float64) {
	if len(lines) == 0 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 3 {
		return
	}
	l1, _ = strconv.ParseFloat(fields[0], 64)
	l5, _ = strconv.ParseFloat(fields[1], 64)
	l15, _ = strconv.ParseFloat(fields[2], 64)
	return
}

func parseUptime(lines []string) time.Duration {
	if len(lines) == 0 {
		return 0
	}
	fields := strings.Fields(lines[0])
	if len(fields) == 0 {
		return 0
	}
	secs, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

func parseNet(lines []string) (rx, tx uint64) {
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" || iface == "" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}
		if v, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
			rx += v
		}
		if v, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
			tx += v
		}
	}
	return
}

func parseDisk(lines []string) (totalKB, usedKB uint64) {
	if len(lines) == 0 {
		return
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 4 {
		return
	}
	totalKB, _ = strconv.ParseUint(fields[1], 10, 64)
	usedKB, _ = strconv.ParseUint(fields[2], 10, 64)
	return
}

type dockerStatsLine struct {
	Container string `json:"Container"`
	Name      string `json:"Name"`
	CPUPerc   string `json:"CPUPerc"`
	MemUsage  string `json:"MemUsage"`
	NetIO     string `json:"NetIO"`
	BlockIO   string `json:"BlockIO"`
	PIDs      string `json:"PIDs"`
}

type dockerPSLine struct {
	ID     string `json:"ID"`
	Names  string `json:"Names"`
	Image  string `json:"Image"`
	Status string `json:"Status"`
	Labels string `json:"Labels"`
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func parsePercent(s string) float64 {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "--" {
		return 0
	}
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	numPart := s[:i]
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	val, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0
	}
	mult := 1.0
	switch unit {
	case "b", "":
		mult = 1
	case "kb":
		mult = 1000
	case "kib":
		mult = 1024
	case "mb":
		mult = 1000 * 1000
	case "mib":
		mult = 1024 * 1024
	case "gb":
		mult = 1000 * 1000 * 1000
	case "gib":
		mult = 1024 * 1024 * 1024
	case "tb":
		mult = 1000 * 1000 * 1000 * 1000
	case "tib":
		mult = 1024 * 1024 * 1024 * 1024
	}
	return uint64(val * mult)
}

func parseIOPair(s string) (a, b uint64) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	return parseSize(parts[0]), parseSize(parts[1])
}

func labelValue(labels, key string) string {
	for _, kv := range strings.Split(labels, ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1]
		}
	}
	return ""
}

func parseContainers(statsLines, psLines []string) []model.Container {
	psByID := map[string]dockerPSLine{}
	for _, line := range psLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var p dockerPSLine
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			continue
		}
		psByID[shortID(p.ID)] = p
	}

	var out []model.Container
	for _, line := range statsLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var st dockerStatsLine
		if err := json.Unmarshal([]byte(line), &st); err != nil {
			continue
		}
		id := shortID(st.Container)
		ps := psByID[id]

		serviceName := labelValue(ps.Labels, "com.docker.swarm.service.name")
		memUsed, memLimit := parseIOPair(st.MemUsage)
		netRx, netTx := parseIOPair(st.NetIO)
		blkRead, blkWrite := parseIOPair(st.BlockIO)
		pids, _ := strconv.Atoi(strings.TrimSpace(st.PIDs))

		out = append(out, model.Container{
			ID:              id,
			Name:            st.Name,
			Image:           ps.Image,
			ServiceName:     serviceName,
			Status:          ps.Status,
			CPUPercent:      parsePercent(st.CPUPerc),
			MemUsageBytes:   memUsed,
			MemLimitBytes:   memLimit,
			NetRxBytes:      netRx,
			NetTxBytes:      netTx,
			BlockReadBytes:  blkRead,
			BlockWriteBytes: blkWrite,
			PIDs:            pids,
		})
	}
	return out
}

func cpuPercentFromDelta(prev, cur cpuCounters) float64 {
	totalDelta := int64(cur.total - prev.total)
	idleDelta := int64(cur.idle - prev.idle)
	if totalDelta <= 0 {
		return 0
	}
	pct := (1 - float64(idleDelta)/float64(totalDelta)) * 100
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct
}

func Parse(output string, prev *Sample) (model.HostStats, []model.Container, Sample) {
	s := splitSections(output)

	cpuAll, cpuCores := parseCPU(s.cpu)
	memTotal, memUsed, swapTotal, swapUsed := parseMem(s.mem)
	l1, l5, l15 := parseLoad(s.load)
	uptime := parseUptime(s.uptime)
	rx, tx := parseNet(s.net)
	diskTotal, diskUsed := parseDisk(s.disk)
	containers := parseContainers(s.dockerStats, s.dockerPS)

	now := time.Now()
	cur := Sample{
		Timestamp: now,
		CPUAll:    cpuAll,
		CPUCores:  cpuCores,
		NetRxTot:  rx,
		NetTxTot:  tx,
	}

	host := model.HostStats{
		MemTotalKB:  memTotal,
		MemUsedKB:   memUsed,
		SwapTotalKB: swapTotal,
		SwapUsedKB:  swapUsed,
		Load1:       l1,
		Load5:       l5,
		Load15:      l15,
		DiskTotalKB: diskTotal,
		DiskUsedKB:  diskUsed,
		Uptime:      uptime,
	}

	host.PerCoreCPU = make([]float64, len(cpuCores))
	if prev != nil {
		elapsed := now.Sub(prev.Timestamp).Seconds()
		host.CPUPercent = cpuPercentFromDelta(prev.CPUAll, cpuAll)
		for i, c := range cpuCores {
			if i < len(prev.CPUCores) {
				host.PerCoreCPU[i] = cpuPercentFromDelta(prev.CPUCores[i], c)
			}
		}
		if elapsed > 0 {
			if cur.NetRxTot >= prev.NetRxTot {
				host.NetRxBytesPerSec = float64(cur.NetRxTot-prev.NetRxTot) / elapsed
			}
			if cur.NetTxTot >= prev.NetTxTot {
				host.NetTxBytesPerSec = float64(cur.NetTxTot-prev.NetTxTot) / elapsed
			}
		}
	}

	return host, containers, cur
}
