package collector

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

var (
	staticOSOnce     sync.Once
	cachedNumCPUs    int
	cachedCPUModel   string
	cachedKernelRel  string
	osStorageCacheMu sync.Mutex
	lastOSStorageRun time.Time
	cachedFS         []model.FilesystemMount
	cachedSANPaths   []model.StoragePathInfo
)

func initStaticOSMetadata() {
	staticOSOnce.Do(func() {
		cachedNumCPUs, cachedCPUModel = readCPUInfo()
		kernelRaw, _ := os.ReadFile("/proc/sys/kernel/osrelease")
		cachedKernelRel = strings.TrimSpace(string(kernelRaw))
		if cachedKernelRel == "" {
			cachedKernelRel = "Linux"
		}
	})
}

// CollectOSHealth gathers comprehensive Linux server health metrics:
// 1. CPU cores, Load Average (1m/5m/15m), Uptime, Kernel version
// 2. Physical RAM, Available RAM, PageCache, Swap usage & vm.swappiness
// 3. Static HugePages (2MB/1GB) & Transparent HugePages (THP) status
// 4. Kernel Async I/O (fs.aio-nr / fs.aio-max-nr), File Descriptors (fs.file-nr), SELinux
// 5. Mounted Filesystems (df -PTm) + Inode exhaustion checks (df -Pi)
// 6. SAN Multipath LUNs & paths (multipath -ll), Fibre Channel HBAs (/sys/class/fc_host), OracleASM disks, and Block Disks (/sys/block)
func CollectOSHealth(ctx context.Context, runner *executor.Runner, env *discovery.DiscoveredEnvironment) ([]model.OSHealthMetric, []model.FilesystemMount, []model.StoragePathInfo) {
	metrics := make([]model.OSHealthMetric, 0, 7)

	// 1. CPU, Load Average & Uptime (static CPU/kernel cached via sync.Once)
	initStaticOSMetadata()
	numCPUs, cpuModel, kernelVer := cachedNumCPUs, cachedCPUModel, cachedKernelRel
	loadavgRaw, _ := os.ReadFile("/proc/loadavg")
	uptimeRaw, _ := os.ReadFile("/proc/uptime")

	load1, load5, load15, procRatio := parseLoadAvg(string(loadavgRaw))
	uptimeStr := parseUptime(string(uptimeRaw))

	loadPct := 0.0
	if numCPUs > 0 {
		loadPct = (load1 / float64(numCPUs)) * 100.0
	}
	loadStatus := "ONLINE"
	loadRec := fmt.Sprintf("Carga saludable (%.2f para %d vCPUs) │ %s", load1, numCPUs, cpuModel)
	if numCPUs > 0 && load1 >= float64(numCPUs)*1.5 {
		loadStatus = "HIGH"
		loadRec = fmt.Sprintf("ALERTA: Saturacion de CPU (Load %.2f > 1.5x %d vCPUs)", load1, numCPUs)
	} else if numCPUs > 0 && load1 >= float64(numCPUs) {
		loadStatus = "WARNING"
		loadRec = fmt.Sprintf("Carga elevada (Load %.2f >= %d vCPUs) │ Revisar sesiones ON CPU", load1, numCPUs)
	}

	metrics = append(metrics, model.OSHealthMetric{
		Category:       "CPU & LOAD",
		Component:      fmt.Sprintf("CPU (%d Cores) & Load Avg", numCPUs),
		CurrentValue:   fmt.Sprintf("Load: %.2f / %.2f / %.2f │ Procs: %s │ Up: %s", load1, load5, load15, procRatio, uptimeStr),
		UsedPct:        clampPct(loadPct),
		Status:         loadStatus,
		Recommendation: loadRec,
	})

	// 2. Physical RAM, Swap & HugePages (/proc/meminfo)
	memRaw, _ := os.ReadFile("/proc/meminfo")
	memMap := parseMemInfo(string(memRaw))

	memTotalKB := memMap["MemTotal"]
	memAvailKB := memMap["MemAvailable"]
	if memAvailKB == 0 {
		memAvailKB = memMap["MemFree"] + memMap["Buffers"] + memMap["Cached"]
	}
	memCachedKB := memMap["Buffers"] + memMap["Cached"] + memMap["SReclaimable"]
	if memTotalKB > 0 {
		memUsedKB := memTotalKB - memAvailKB
		if memUsedKB < 0 {
			memUsedKB = 0
		}
		ramPct := float64(memUsedKB) * 100.0 / float64(memTotalKB)
		ramStatus := "ONLINE"
		ramRec := fmt.Sprintf("Disponible: %.1f GB │ PageCache/Buffers: %.1f GB", float64(memAvailKB)/1048576.0, float64(memCachedKB)/1048576.0)
		if ramPct >= 92.0 {
			ramStatus = "HIGH"
			ramRec = "ALERTA: Presion critica de RAM (<8% disponible) │ Riesgo de swapping/OOM"
		} else if ramPct >= 82.0 {
			ramStatus = "WARNING"
			ramRec = fmt.Sprintf("Uso de RAM alto (%.1f%%) │ Libre real: %.1f GB", ramPct, float64(memAvailKB)/1048576.0)
		}
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "MEMORIA RAM",
			Component:      "Memoria Fisica Servidor",
			CurrentValue:   fmt.Sprintf("Usada: %.1f GB / %.1f GB (Disp: %.1f GB)", float64(memUsedKB)/1048576.0, float64(memTotalKB)/1048576.0, float64(memAvailKB)/1048576.0),
			UsedPct:        clampPct(ramPct),
			Status:         ramStatus,
			Recommendation: ramRec,
		})
	}

	// 3. Swap & vm.swappiness
	swapTotalKB := memMap["SwapTotal"]
	swapFreeKB := memMap["SwapFree"]
	swappinessRaw, _ := os.ReadFile("/proc/sys/vm/swappiness")
	swappiness := strings.TrimSpace(string(swappinessRaw))
	if swappiness == "" {
		swappiness = "60"
	}
	if swapTotalKB > 0 {
		swapUsedKB := swapTotalKB - swapFreeKB
		swapPct := float64(swapUsedKB) * 100.0 / float64(swapTotalKB)
		swapStatus := "ONLINE"
		swapRec := fmt.Sprintf("vm.swappiness=%s (Oracle recomienda <= 10)", swappiness)
		if swapPct >= 50.0 {
			swapStatus = "HIGH"
			swapRec = fmt.Sprintf("ALERTA: Paginacion activa en Swap (%.1f%%) │ vm.swappiness=%s", swapPct, swappiness)
		} else if swapPct >= 15.0 {
			swapStatus = "WARNING"
			swapRec = fmt.Sprintf("Swap en uso (%.0f MB) │ Verifique vm.swappiness=%s", float64(swapUsedKB)/1024.0, swappiness)
		}
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "MEMORIA SWAP",
			Component:      "Swap & vm.swappiness",
			CurrentValue:   fmt.Sprintf("Usado: %.1f GB / %.1f GB (swappiness=%s)", float64(swapUsedKB)/1048576.0, float64(swapTotalKB)/1048576.0, swappiness),
			UsedPct:        clampPct(swapPct),
			Status:         swapStatus,
			Recommendation: swapRec,
		})
	} else {
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "MEMORIA SWAP",
			Component:      "Swap & vm.swappiness",
			CurrentValue:   fmt.Sprintf("Sin Swap activo (0 MB) │ vm.swappiness=%s", swappiness),
			UsedPct:        0,
			Status:         "WARNING",
			Recommendation: "Oracle recomienda configurar particion/LV de Swap en Linux",
		})
	}

	// 4. Static HugePages & Transparent HugePages (THP)
	hpTotal := memMap["HugePages_Total"]
	hpFree := memMap["HugePages_Free"]
	hpRsvd := memMap["HugePages_Rsvd"]
	hpSizeKB := memMap["Hugepagesize"]
	if hpSizeKB == 0 {
		hpSizeKB = 2048
	}
	if hpTotal > 0 {
		hpUsed := hpTotal - hpFree + hpRsvd
		if hpUsed > hpTotal {
			hpUsed = hpTotal
		}
		hpPct := float64(hpUsed) * 100.0 / float64(hpTotal)
		hpTotalGB := float64(hpTotal*hpSizeKB) / 1048576.0
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "HUGEPAGES",
			Component:      fmt.Sprintf("HugePages Estáticas (%dK)", hpSizeKB),
			CurrentValue:   fmt.Sprintf("Total: %d (%.1f GB) │ En Uso/Rsvd: %d │ Libres: %d", hpTotal, hpTotalGB, hpUsed, hpFree),
			UsedPct:        clampPct(hpPct),
			Status:         "ONLINE",
			Recommendation: "Óptimo: SGA fijada en HugePages físicas (Sin PageTables overhead)",
		})
	} else {
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "HUGEPAGES",
			Component:      "HugePages Estáticas (2MB)",
			CurrentValue:   "HugePages_Total: 0 (SGA en páginas estándar de 4 KB)",
			UsedPct:        0,
			Status:         "INFO",
			Recommendation: "Para SGA > 8 GB se recomienda configurar vm.nr_hugepages en /etc/sysctl.conf",
		})
	}

	thpStatus, thpVal, thpRec := checkTHPStatus()
	metrics = append(metrics, model.OSHealthMetric{
		Category:       "TRANSP. HP",
		Component:      "Transparent HugePages (THP)",
		CurrentValue:   thpVal,
		UsedPct:        0,
		Status:         thpStatus,
		Recommendation: thpRec,
	})

	// 5. Kernel Async I/O (fs.aio-nr / fs.aio-max-nr)
	aioNrRaw, _ := os.ReadFile("/proc/sys/fs/aio-nr")
	aioMaxRaw, _ := os.ReadFile("/proc/sys/fs/aio-max-nr")
	aioNr, _ := strconv.ParseInt(strings.TrimSpace(string(aioNrRaw)), 10, 64)
	aioMax, _ := strconv.ParseInt(strings.TrimSpace(string(aioMaxRaw)), 10, 64)
	if aioMax > 0 {
		aioPct := float64(aioNr) * 100.0 / float64(aioMax)
		aioSt := "ONLINE"
		aioRec := "I/O Asíncrono de kernel saludable para DBWR/LGWR (fs.aio-max-nr OK)"
		if aioPct >= 85.0 {
			aioSt = "HIGH"
			aioRec = "ALERTA: Agotamiento de slots AIO (Riesgo ORA-27090) │ Ampliar fs.aio-max-nr"
		} else if aioMax < 1048576 {
			aioSt = "WARNING"
			aioRec = "Oracle recomienda fs.aio-max-nr >= 1048576 (1M)"
		}
		metrics = append(metrics, model.OSHealthMetric{
			Category:       "KERNEL AIO",
			Component:      "Async I/O (fs.aio-nr)",
			CurrentValue:   fmt.Sprintf("Activos: %d / Max: %d (%.1f%%)", aioNr, aioMax, aioPct),
			UsedPct:        clampPct(aioPct),
			Status:         aioSt,
			Recommendation: aioRec,
		})
	}

	// 6. File Descriptors (fs.file-nr), Kernel & SELinux
	fileNrRaw, _ := os.ReadFile("/proc/sys/fs/file-nr")
	fileFields := strings.Fields(string(fileNrRaw))
	var fileOpen, fileMax int64
	if len(fileFields) >= 3 {
		fileOpen, _ = strconv.ParseInt(fileFields[0], 10, 64)
		fileMax, _ = strconv.ParseInt(fileFields[2], 10, 64)
	}
	selinuxSt := readSELinuxMode()
	filePct := 0.0
	if fileMax > 0 {
		filePct = float64(fileOpen) * 100.0 / float64(fileMax)
	}
	metrics = append(metrics, model.OSHealthMetric{
		Category:       "KERNEL & OS",
		Component:      "File Handles & Kernel",
		CurrentValue:   fmt.Sprintf("FD: %d / %d │ Kernel: %s │ SELinux: %s", fileOpen, fileMax, kernelVer, selinuxSt),
		UsedPct:        clampPct(filePct),
		Status:         "ONLINE",
		Recommendation: "Descriptores de fichero (fs.file-max) y kernel operativos",
	})

	// 7 & 8. Filesystems, Inodes (df -PTm + df -Pi) & SAN Multipath/FC HBA paths (cached with 30s TTL to reduce OS subprocess forks)
	now := time.Now()
	osStorageCacheMu.Lock()
	needStorageRefresh := now.Sub(lastOSStorageRun) >= 30*time.Second || (len(cachedFS) == 0 && len(cachedSANPaths) == 0)
	osStorageCacheMu.Unlock()

	if needStorageRefresh {
		var fsList []model.FilesystemMount
		if runner != nil {
			envMap := map[string]string{"LC_ALL": "C"}
			dfCtx1, cancel1 := context.WithTimeout(ctx, 3*time.Second)
			dfBlocksOut, _ := runner.RunCommand(dfCtx1, nil, false, envMap, nil, "df", "-PTm")
			cancel1()
			dfCtx2, cancel2 := context.WithTimeout(ctx, 3*time.Second)
			dfInodesOut, _ := runner.RunCommand(dfCtx2, nil, false, envMap, nil, "df", "-Pi")
			cancel2()
			fsList = ParseDFOutput(dfBlocksOut, dfInodesOut)
		}
		sanList := collectStoragePaths(ctx, runner)
		osStorageCacheMu.Lock()
		cachedFS = fsList
		cachedSANPaths = sanList
		lastOSStorageRun = now
		osStorageCacheMu.Unlock()
	}

	osStorageCacheMu.Lock()
	filesystems := cachedFS
	storagePaths := cachedSANPaths
	osStorageCacheMu.Unlock()

	return metrics, filesystems, storagePaths
}

func readCPUInfo() (int, string) {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1, "Linux CPU"
	}
	count := 0
	modelName := ""
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			count++
		} else if modelName == "" && strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				modelName = strings.TrimSpace(parts[1])
			}
		}
	}
	if count == 0 {
		count = 1
	}
	if modelName == "" {
		modelName = "Server Processor"
	}
	return count, modelName
}

func parseLoadAvg(raw string) (float64, float64, float64, string) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) < 4 {
		return 0, 0, 0, "0/0"
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, fields[3]
}

func parseUptime(raw string) string {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "N/A"
	}
	secFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "N/A"
	}
	totalSec := int64(secFloat)
	days := totalSec / 86400
	hours := (totalSec % 86400) / 3600
	mins := (totalSec % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %02dh %02dm", days, hours, mins)
	}
	return fmt.Sprintf("%02dh %02dm", hours, mins)
}

func parseMemInfo(raw string) map[string]int64 {
	res := make(map[string]int64)
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valFields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(valFields) > 0 {
			v, err := strconv.ParseInt(valFields[0], 10, 64)
			if err == nil {
				res[key] = v
			}
		}
	}
	return res
}

func checkTHPStatus() (string, string, string) {
	paths := []string{
		"/sys/kernel/mm/transparent_hugepage/enabled",
		"/sys/kernel/mm/redhat_transparent_hugepage/enabled",
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(raw))
		if strings.Contains(val, "[never]") {
			return "ONLINE", val, "Óptimo: THP desactivado ([never]) según requisito Oracle RAC/DB"
		}
		if strings.Contains(val, "[always]") {
			return "WARNING", val, "ALERTA: THP activo ([always]) │ Riesgo de bloqueos/evictions en Oracle (Desactivar con transparent_hugepage=never)"
		}
		return "WARNING", val, "Se recomienda desactivar THP completamente ([never]) en servidores Oracle"
	}
	return "ONLINE", "[never] (No presente)", "THP no interfiere con la memoria SGA"
}

func readSELinuxMode() string {
	raw, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return "Disabled"
	}
	if strings.TrimSpace(string(raw)) == "1" {
		return "Enforcing"
	}
	return "Permissive"
}

// ParseDFOutput parses `df -PTm` (block usage in MB) and `df -Pi` (inode usage %) into FilesystemMount slices.
func ParseDFOutput(dfBlocksOut, dfInodesOut string) []model.FilesystemMount {
	inodeMap := make(map[string]float64)
	for i, line := range strings.Split(dfInodesOut, "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line))
		// POSIX `df -Pi`: Filesystem Inodes IUsed IFree IUse% Mounted_on
		if len(fields) >= 6 {
			mount := fields[len(fields)-1]
			pctStr := strings.TrimSuffix(fields[len(fields)-2], "%")
			if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
				inodeMap[mount] = pct
			}
		}
	}

	ignoredFSTypes := map[string]bool{
		"proc": true, "sysfs": true, "devtmpfs": true, "cgroup": true, "cgroup2": true,
		"overlay": true, "squashfs": true, "autofs": true, "pstore": true, "mqueue": true,
		"hugetlbfs": true, "debugfs": true, "tracefs": true, "securityfs": true,
		"configfs": true, "fusectl": true, "binfmt_misc": true, "rpc_pipefs": true,
		"efivarfs": true, "devpts": true, "iso9660": true,
	}

	var mounts []model.FilesystemMount
	for i, line := range strings.Split(dfBlocksOut, "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(line))
		// POSIX `df -PTm`: Filesystem Type 1048576-blocks Used Available Capacity Mounted_on
		if len(fields) < 7 {
			continue
		}
		dev := fields[0]
		fsType := fields[1]
		if ignoredFSTypes[fsType] {
			continue
		}
		mount := fields[6]
		if strings.HasPrefix(mount, "/run/user") || strings.HasPrefix(mount, "/sys") || strings.HasPrefix(mount, "/proc") || mount == "/boot/efi" {
			continue
		}
		if fsType == "tmpfs" && mount != "/dev/shm" && mount != "/tmp" && mount != "/run" {
			continue
		}

		totalMB, _ := strconv.Atoi(fields[2])
		usedMB, _ := strconv.Atoi(fields[3])
		availMB, _ := strconv.Atoi(fields[4])
		pctStr := strings.TrimSuffix(fields[5], "%")
		usedPct, _ := strconv.ParseFloat(pctStr, 64)
		if totalMB > 0 && usedPct == 0 && usedMB > 0 {
			usedPct = float64(usedMB) * 100.0 / float64(totalMB)
		}
		inodePct := inodeMap[mount]

		status := "ONLINE"
		if usedPct >= 90.0 || inodePct >= 90.0 {
			status = "HIGH (CRITICAL)"
		} else if usedPct >= 80.0 || inodePct >= 80.0 {
			status = "WARNING"
		}

		mounts = append(mounts, model.FilesystemMount{
			MountPoint:   mount,
			Device:       dev,
			FSType:       fsType,
			TotalMB:      totalMB,
			UsedMB:       usedMB,
			AvailMB:      availMB,
			UsedPct:      usedPct,
			InodeUsedPct: inodePct,
			Status:       status,
		})
	}

	sort.SliceStable(mounts, func(i, j int) bool {
		// Prioritize Oracle mounts (/u01, /u02, /ora*, /dev/shm, /) first
		pI := mountPriority(mounts[i].MountPoint)
		pJ := mountPriority(mounts[j].MountPoint)
		if pI != pJ {
			return pI < pJ
		}
		return mounts[i].MountPoint < mounts[j].MountPoint
	})

	return mounts
}

func mountPriority(m string) int {
	switch {
	case strings.HasPrefix(m, "/u0") || strings.HasPrefix(m, "/ora") || strings.HasPrefix(m, "/grid"):
		return 1
	case m == "/":
		return 2
	case m == "/dev/shm":
		return 3
	case m == "/tmp" || m == "/var":
		return 4
	default:
		return 5
	}
}

var (
	mpathHeaderRe = regexp.MustCompile(`^([^\s]+)\s+(?:\(([^)]+)\)\s+)?(dm-\d+)\s*(.*)$`)
	mpathChildRe  = regexp.MustCompile(`(\d+:\d+:\d+:\d+)\s+([a-zA-Z0-9_]+)\s+\d+:\d+\s+(.*)$`)
)

// ParseMultipathOutput parses `multipath -ll` output to verify that all SAN paths per LUN are active and online.
func ParseMultipathOutput(out string) []model.StoragePathInfo {
	var results []model.StoragePathInfo
	lines := strings.Split(out, "\n")

	var curr *model.StoragePathInfo
	var totalPaths, activePaths, failedPaths int
	var pathDevs []string

	flushCurr := func() {
		if curr == nil {
			return
		}
		if totalPaths > 0 {
			if failedPaths == 0 && activePaths == totalPaths {
				curr.PathsSummary = fmt.Sprintf("%d/%d ACTIVE", activePaths, totalPaths)
				curr.Status = "ONLINE"
			} else if activePaths > 0 {
				curr.PathsSummary = fmt.Sprintf("%d/%d DEGRADED (%d DOWN)", activePaths, totalPaths, failedPaths)
				curr.Status = "WARNING"
			} else {
				curr.PathsSummary = fmt.Sprintf("0/%d PATHS DOWN", totalPaths)
				curr.Status = "OFFLINE"
			}
			curr.PathDetails = strings.Join(pathDevs, ", ")
		} else if curr.PathsSummary == "" {
			curr.PathsSummary = "ONLINE"
			curr.Status = "ONLINE"
		}
		results = append(results, *curr)
		curr = nil
		totalPaths = 0
		activePaths = 0
		failedPaths = 0
		pathDevs = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.Contains(line, "dm-") && !strings.HasPrefix(line, "|") && !strings.HasPrefix(line, "`") && !strings.HasPrefix(line, "size=") {
			if m := mpathHeaderRe.FindStringSubmatch(line); m != nil {
				flushCurr()
				name := m[1]
				wwid := m[2]
				dmDev := m[3]
				vendor := strings.TrimSpace(m[4])
				wwidOrVendor := vendor
				if wwid != "" {
					if vendor != "" {
						wwidOrVendor = fmt.Sprintf("%s (%s)", vendor, wwid)
					} else {
						wwidOrVendor = wwid
					}
				}
				if wwidOrVendor == "" {
					wwidOrVendor = dmDev
				}
				curr = &model.StoragePathInfo{
					Name:        fmt.Sprintf("%s (%s)", name, dmDev),
					Type:        "MULTIPATH SAN",
					WWIDOrModel: wwidOrVendor,
					Size:        "-",
				}
				continue
			}
		}

		if curr != nil && strings.HasPrefix(line, "size=") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				curr.Size = strings.TrimPrefix(fields[0], "size=")
			}
			continue
		}

		if curr != nil {
			if m := mpathChildRe.FindStringSubmatch(line); m != nil {
				hctl := m[1]
				dev := m[2]
				stateStr := strings.ToLower(strings.TrimSpace(m[3]))
				totalPaths++
				if strings.Contains(stateStr, "failed") || strings.Contains(stateStr, "faulty") || strings.Contains(stateStr, "offline") || strings.Contains(stateStr, "shaky") {
					failedPaths++
					pathDevs = append(pathDevs, fmt.Sprintf("%s[%s:FAULTY!]", dev, hctl))
				} else {
					activePaths++
					pathDevs = append(pathDevs, fmt.Sprintf("%s[%s:OK]", dev, hctl))
				}
			}
		}
	}
	flushCurr()
	return results
}

func collectStoragePaths(ctx context.Context, runner *executor.Runner) []model.StoragePathInfo {
	var paths []model.StoragePathInfo

	// 1. Fibre Channel HBA Host Ports (/sys/class/fc_host/host*)
	fcHosts, _ := filepath.Glob("/sys/class/fc_host/host*")
	sort.Strings(fcHosts)
	for _, hPath := range fcHosts {
		hostName := filepath.Base(hPath)
		portStateRaw, _ := os.ReadFile(filepath.Join(hPath, "port_state"))
		speedRaw, _ := os.ReadFile(filepath.Join(hPath, "speed"))
		wwpnRaw, _ := os.ReadFile(filepath.Join(hPath, "port_name"))
		portState := strings.TrimSpace(string(portStateRaw))
		speed := strings.TrimSpace(string(speedRaw))
		wwpn := strings.TrimSpace(string(wwpnRaw))
		if portState == "" {
			portState = "Online"
		}
		st := "ONLINE"
		if strings.Contains(strings.ToLower(portState), "down") || strings.Contains(strings.ToLower(portState), "offline") || strings.Contains(strings.ToLower(portState), "error") {
			st = "OFFLINE"
		}
		paths = append(paths, model.StoragePathInfo{
			Name:         "HBA " + hostName,
			Type:         "FC_HBA PORT",
			WWIDOrModel:  "WWPN: " + wwpn,
			Size:         speed,
			PathsSummary: strings.ToUpper(portState),
			PathDetails:  fmt.Sprintf("Fibre Channel HBA %s │ Port State: %s │ Speed: %s", hostName, portState, speed),
			Status:       st,
		})
	}

	// 2. Multipath SAN LUNs (multipath -ll)
	if runner != nil {
		for _, bin := range []string{"/sbin/multipath", "/usr/sbin/multipath"} {
			if _, err := os.Stat(bin); err == nil {
				mpCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				out, err := runner.RunCommand(mpCtx, nil, false, map[string]string{"LC_ALL": "C"}, nil, bin, "-ll")
				cancel()
				if err == nil && strings.TrimSpace(out) != "" {
					mpList := ParseMultipathOutput(out)
					paths = append(paths, mpList...)
				}
				break
			}
		}
	}

	// 3. Oracle ASM Disks (/dev/oracleasm/disks/* or /dev/oracleafd/disks/*)
	for _, asmDir := range []string{"/dev/oracleasm/disks", "/dev/oracleafd/disks"} {
		entries, err := os.ReadDir(asmDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			dPath := filepath.Join(asmDir, e.Name())
			paths = append(paths, model.StoragePathInfo{
				Name:         e.Name(),
				Type:         "ASM_DISK",
				WWIDOrModel:  dPath,
				Size:         "ASM LUN",
				PathsSummary: "ONLINE (READY)",
				PathDetails:  "Dispositivo de bloque ASM visible en " + dPath,
				Status:       "ONLINE",
			})
		}
	}

	// 4. Physical Block Disks (/sys/block/*)
	blockDirs, _ := filepath.Glob("/sys/block/*")
	sort.Strings(blockDirs)
	for _, bDir := range blockDirs {
		devName := filepath.Base(bDir)
		if strings.HasPrefix(devName, "loop") || strings.HasPrefix(devName, "ram") ||
			strings.HasPrefix(devName, "sr") || strings.HasPrefix(devName, "fd") ||
			strings.HasPrefix(devName, "zram") || strings.HasPrefix(devName, "dm-") {
			continue
		}
		sectorsRaw, _ := os.ReadFile(filepath.Join(bDir, "size"))
		sectors, _ := strconv.ParseInt(strings.TrimSpace(string(sectorsRaw)), 10, 64)
		if sectors <= 0 {
			continue
		}
		sizeGB := float64(sectors*512) / (1024.0 * 1024.0 * 1024.0)

		vendorRaw, _ := os.ReadFile(filepath.Join(bDir, "device", "vendor"))
		modelRaw, _ := os.ReadFile(filepath.Join(bDir, "device", "model"))
		stateRaw, _ := os.ReadFile(filepath.Join(bDir, "device", "state"))
		schedRaw, _ := os.ReadFile(filepath.Join(bDir, "queue", "scheduler"))

		vendorModel := strings.TrimSpace(strings.TrimSpace(string(vendorRaw)) + " " + strings.TrimSpace(string(modelRaw)))
		if vendorModel == "" {
			vendorModel = "Block Device"
		}
		devState := strings.TrimSpace(string(stateRaw))
		if devState == "" {
			devState = "running"
		}
		sched := extractBracketed(strings.TrimSpace(string(schedRaw)))

		st := "ONLINE"
		summary := "1/1 PATH OK (" + strings.ToUpper(devState) + ")"
		if strings.Contains(strings.ToLower(devState), "offline") || strings.Contains(strings.ToLower(devState), "fault") || strings.Contains(strings.ToLower(devState), "blocked") {
			st = "OFFLINE"
			summary = "PATH OFFLINE (" + strings.ToUpper(devState) + ")"
		}

		paths = append(paths, model.StoragePathInfo{
			Name:         "/dev/" + devName,
			Type:         "BLOCK_DISK",
			WWIDOrModel:  vendorModel,
			Size:         fmt.Sprintf("%.1f GB", sizeGB),
			PathsSummary: summary,
			PathDetails:  fmt.Sprintf("Estado: %s │ I/O Scheduler: %s", devState, sched),
			Status:       st,
		})
	}

	return paths
}

func extractBracketed(s string) string {
	start := strings.IndexByte(s, '[')
	end := strings.IndexByte(s, ']')
	if start >= 0 && end > start {
		return s[start+1 : end]
	}
	if s == "" {
		return "default"
	}
	return s
}

func clampPct(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
