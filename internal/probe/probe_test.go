package probe

import (
	"strings"
	"testing"
	"time"
)

func sampleOutput(cpuTotal, cpuIdle uint64, rx, tx uint64) string {
	other := cpuTotal - cpuIdle
	cpuLine := "cpu  " + itoa(other) + " 0 0 " + itoa(cpuIdle) + " 0 0 0 0"
	core0 := "cpu0 " + itoa(other) + " 0 0 " + itoa(cpuIdle) + " 0 0 0 0"

	var b strings.Builder
	b.WriteString("@@CPU\n")
	b.WriteString(cpuLine + "\n")
	b.WriteString(core0 + "\n")
	b.WriteString("@@MEM\n")
	b.WriteString("MemTotal:       16000000 kB\n")
	b.WriteString("MemAvailable:    4000000 kB\n")
	b.WriteString("SwapTotal:       2000000 kB\n")
	b.WriteString("SwapFree:        2000000 kB\n")
	b.WriteString("@@LOAD\n")
	b.WriteString("0.10 0.20 0.30 1/234 5678\n")
	b.WriteString("@@UPTIME\n")
	b.WriteString("12345.67 0.00\n")
	b.WriteString("@@NET\n")
	b.WriteString("Inter-|   Receive\n")
	b.WriteString("lo: 999 0 0 0 0 0 0 0 999 0 0 0 0 0 0 0\n")
	b.WriteString("eth0: " + itoa(rx) + " 0 0 0 0 0 0 0 " + itoa(tx) + " 0 0 0 0 0 0 0\n")
	b.WriteString("@@DISK\n")
	b.WriteString("/dev/sda1 100000 40000 60000 40% /\n")
	b.WriteString("@@DOCKERSTATS\n")
	b.WriteString(`{"Container":"abcdef123456789","Name":"web.1.xyz","CPUPerc":"12.50%","MemUsage":"100MiB / 2GiB","NetIO":"1kB / 2kB","BlockIO":"3MB / 4MB","PIDs":"5"}` + "\n")
	b.WriteString("@@DOCKERPS\n")
	b.WriteString(`{"ID":"abcdef123456789","Names":"web.1.xyz","Image":"nginx:latest","Status":"Up 2 hours","Labels":"com.docker.swarm.service.name=web,other=x"}` + "\n")
	return b.String()
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func TestParseFirstSample(t *testing.T) {
	out := sampleOutput(1000, 500, 1_000_000, 2_000_000)
	host, containers, sample := Parse(out, nil)

	if host.CPUPercent != 0 {
		t.Errorf("expected 0%% CPU on first sample, got %v", host.CPUPercent)
	}
	if host.MemTotalKB != 16000000 || host.MemUsedKB != 12000000 {
		t.Errorf("unexpected mem: total=%d used=%d", host.MemTotalKB, host.MemUsedKB)
	}
	if host.SwapTotalKB != 2000000 || host.SwapUsedKB != 0 {
		t.Errorf("unexpected swap: total=%d used=%d", host.SwapTotalKB, host.SwapUsedKB)
	}
	if host.Load1 != 0.10 || host.Load5 != 0.20 || host.Load15 != 0.30 {
		t.Errorf("unexpected load: %v %v %v", host.Load1, host.Load5, host.Load15)
	}
	if host.DiskTotalKB != 100000 || host.DiskUsedKB != 40000 {
		t.Errorf("unexpected disk: total=%d used=%d", host.DiskTotalKB, host.DiskUsedKB)
	}
	if host.Uptime != time.Duration(12345.67*float64(time.Second)) {
		t.Errorf("unexpected uptime: %v", host.Uptime)
	}
	if sample.NetRxTot != 1_000_000 || sample.NetTxTot != 2_000_000 {
		t.Errorf("unexpected net totals: rx=%d tx=%d", sample.NetRxTot, sample.NetTxTot)
	}

	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	c := containers[0]
	if c.ServiceName != "web" {
		t.Errorf("expected service name 'web', got %q", c.ServiceName)
	}
	if c.CPUPercent != 12.5 {
		t.Errorf("expected CPU%% 12.5, got %v", c.CPUPercent)
	}
	if c.MemUsageBytes == 0 || c.MemLimitBytes == 0 {
		t.Errorf("expected non-zero mem usage/limit, got %d/%d", c.MemUsageBytes, c.MemLimitBytes)
	}
	if c.Image != "nginx:latest" {
		t.Errorf("expected image nginx:latest, got %q", c.Image)
	}
}

func TestParseSecondSampleComputesDeltas(t *testing.T) {
	first := sampleOutput(1000, 500, 1_000_000, 2_000_000)
	_, _, prevSample := Parse(first, nil)
	prevSample.Timestamp = time.Now().Add(-2 * time.Second)

	second := sampleOutput(3000, 1000, 1_500_000, 2_400_000)
	host, _, sample := Parse(second, &prevSample)

	if host.CPUPercent < 74 || host.CPUPercent > 76 {
		t.Errorf("expected ~75%% CPU, got %v", host.CPUPercent)
	}
	if len(host.PerCoreCPU) != 1 || host.PerCoreCPU[0] < 74 || host.PerCoreCPU[0] > 76 {
		t.Errorf("expected ~75%% core0 CPU, got %v", host.PerCoreCPU)
	}
	if host.NetRxBytesPerSec <= 0 || host.NetTxBytesPerSec <= 0 {
		t.Errorf("expected positive net rates, got rx=%v tx=%v", host.NetRxBytesPerSec, host.NetTxBytesPerSec)
	}
	if sample.NetRxTot != 1_500_000 {
		t.Errorf("unexpected rx total: %d", sample.NetRxTot)
	}
}
