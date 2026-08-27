// Package system collects host information for the daemon's SystemInfo RPC.
package system

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"

	"homectl/internal/shared/pb"
)

// cpuSampleWindow is how long we sample CPU usage over for a single
// SystemInfo call. Short enough to keep the RPC responsive, long enough to
// produce a meaningful instantaneous reading.
const cpuSampleWindow = 200 * time.Millisecond

// Collect gathers a snapshot of the host's hostname, OS, uptime, CPU, RAM
// and disk usage.
func Collect() (*pb.SystemInfoResponse, error) {
	hostInfo, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("collect host info: %w", err)
	}

	cpuCores, err := cpu.Counts(true)
	if err != nil {
		return nil, fmt.Errorf("collect cpu core count: %w", err)
	}

	cpuModel := ""
	if infos, err := cpu.Info(); err == nil && len(infos) > 0 {
		cpuModel = infos[0].ModelName
	}

	cpuPercents, err := cpu.Percent(cpuSampleWindow, false)
	if err != nil {
		return nil, fmt.Errorf("collect cpu usage: %w", err)
	}
	cpuUsage := 0.0
	if len(cpuPercents) > 0 {
		cpuUsage = cpuPercents[0]
	}

	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("collect memory info: %w", err)
	}

	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("collect disk info: %w", err)
	}

	return &pb.SystemInfoResponse{
		Hostname:        hostInfo.Hostname,
		Os:              hostInfo.OS,
		PlatformVersion: hostInfo.PlatformVersion,
		UptimeSeconds:   hostInfo.Uptime,
		CpuModel:        cpuModel,
		CpuCores:        uint32(cpuCores),
		CpuUsagePercent: cpuUsage,
		MemTotalBytes:   vmem.Total,
		MemUsedBytes:    vmem.Used,
		DiskTotalBytes:  diskUsage.Total,
		DiskUsedBytes:   diskUsage.Used,
	}, nil
}
