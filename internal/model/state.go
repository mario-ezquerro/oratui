package model

import "time"

type TopologyMode string

const (
	ModeRAC     TopologyMode = "ORACLE RAC"
	ModeRestart TopologyMode = "ORACLE RESTART"
	ModeSingle  TopologyMode = "SINGLE INSTANCE"
)

type BorderStyle string

const (
	BorderMixedUnicode  BorderStyle = "UNICODE MIXTO (Simple ┌─┐ + Doble ╔═╗)"
	BorderDoubleUnicode BorderStyle = "UNICODE DOBLE (╔═╗ ║ ╚═╝)"
	BorderSingleUnicode BorderStyle = "UNICODE SIMPLE (┌─┐ │ └─┘)"
	BorderClassicASCII  BorderStyle = "ASCII CLASICO (+ - = |)"
)

type ClusterNode struct {
	ID           int
	Name         string
	State        string // ONLINE, OFFLINE, DRAINED
	Role         string // HUB, LEAF, STANDALONE
	VIP          string
	VIPState     string
	Interconnect string
	CRS          string // ONLINE, OFFLINE
	CSSD         string
	EVMD         string
	ASM          string
}

type PDBInfo struct {
	ConID      int
	Name       string
	OpenMode   string // READ WRITE, READ ONLY, MOUNTED, RESTRICTED
	Restricted string
	SizeGB     float64
	InstName   string
}

type InstanceInfo struct {
	InstID       int
	InstName     string
	DBName       string
	DBUniqueName string
	NodeName     string
	Version      string
	Status       string // OPEN, MOUNTED, STARTED, DOWN
	OpenMode     string // READ WRITE, READ ONLY WITH APPLY
	Role         string // PRIMARY, PHYSICAL STANDBY
	LogMode      string // ARCHIVELOG, NOARCHIVELOG
	FlashbackOn  string // YES, NO
	Uptime       string
	SGAMB        int
	PGAMB        int
	FRAUsedPct   int
	DGLag        string
}

type RACService struct {
	Name       string
	DBName     string
	State      string // ONLINE, OFFLINE
	Preferred  string
	Available  string
	ActiveInst string
	PDB        string
}

type UserSession struct {
	InstID          int
	SID             int
	Serial          int
	Username        string
	Status          string // ACTIVE, INACTIVE, KILLED
	Schema          string
	Machine         string
	Program         string
	SQLID           string
	WaitClass       string
	WaitEvent       string
	ElapsedSec      int
	BlockingSession string
	LogonTime       string
	PGAMB           float64
	SQLText         string
}

type DatafileInfo struct {
	FileID     int
	FileName   string
	Tablespace string
	SizeMB     int
	MaxSizeMB  int
	AutoExtend string // YES, NO
	Status     string // AVAILABLE, ONLINE
}

type TablespaceInfo struct {
	Name       string
	Type       string // PERMANENT, UNDO, TEMPORARY
	Status     string // ONLINE, OFFLINE, READ ONLY
	AllocMB    int    // Currently allocated size on disk (SUM(bytes))
	TotalMB    int    // Maximum capacity considering AUTOEXTEND (TOTAL_MAX)
	UsedMB     int    // Actual space consumed (AllocMB - FreeAllocMB)
	FreeMB     int    // Real remaining free capacity (TotalMB - UsedMB)
	UsedPct    float64
	AutoExtend string // YES, NO
	Datafiles  int
}

type TableSegment struct {
	Owner        string
	SegmentName  string
	SegmentType  string // TABLE, TABLE PARTITION, INDEX
	Tablespace   string
	SizeMB       int
	NumRows      int64
	LastAnalyzed string
	StatsStatus  string // FRESH, STALE, MISSING
	Fragmented   string // LOW, MEDIUM, HIGH (28%)
}

type InvalidObject struct {
	Owner      string
	ObjectName string
	ObjectType string // PACKAGE BODY, VIEW, PROCEDURE, TRIGGER
	Status     string // INVALID
	LastDDL    string
}

type ASMDiskgroup struct {
	Name         string
	State        string // CONNECTED, MOUNTED, DISMOUNTED
	Type         string // NORMAL, HIGH, EXTERN
	TotalMB      int
	FreeMB       int
	UsableMB     int
	UsedPct      float64
	OfflineDisks int
}

type ListenerServiceHandler struct {
	ListenerName string
	ServiceName  string
	InstanceName string
	Status       string // READY, UNKNOWN, BLOCKED, RESTRICTED
	Handlers     int
}

type ListenerInfo struct {
	Name           string
	Type           string // LOCAL, SCAN
	Node           string
	State          string // ONLINE, OFFLINE (tnslsnr process state)
	ReadyStatus    string // READY, NO READY (Solo UNKNOWN), BLOCKED, SIN SERVICIOS, OFFLINE
	Endpoint       string
	Services       int
	ReadyCount     int
	UnknownCount   int
	BlockedCount   int
	ServiceSummary string
	ServiceList    []ListenerServiceHandler
}

type RMANBackupJob struct {
	SessionKey int
	InputType  string // DB FULL, DB INCR L0, DB INCR L1, ARCHIVELOG
	Status     string // COMPLETED, RUNNING, FAILED, COMPLETED WITH WARNINGS
	StartTime  string
	Duration   string
	SizeGB     float64
	DeviceType string // DISK (+FRA), SBT_TAPE
}

type RestorePoint struct {
	Name        string
	SCN         int64
	Time        string
	Guaranteed  string // YES, NO
	StorageSize string
}

type DataGuardStandby struct {
	DBUniqueName string
	Role         string // PHYSICAL STANDBY
	Host         string
	OpenMode     string // READ ONLY WITH APPLY
	TransportLag string // 0 seconds
	ApplyLag     string // 0 seconds
	MRPStatus    string // APPLYING_LOG (ACTIVE)
	Switchover   string // TO STANDBY / READY
}

type AlertLogEntry struct {
	Timestamp string
	Instance  string
	Severity  string // CRITICAL, WARNING, INFO
	ErrorCode string // ORA-01654, ORA-00060, CRS-2401, etc.
	Message   string
}

type OSHealthMetric struct {
	Category       string  // CPU & LOAD, MEMORIA RAM, SWAP, HUGEPAGES, THP, KERNEL AIO, SHM / IPC, LIMITES / NTP
	Component      string  // Nombre del subsistema comprobado
	CurrentValue   string  // Valor actual detallado
	UsedPct        float64 // Porcentaje de uso (o -1 si no aplica barra)
	Status         string  // ONLINE, OK, WARNING, HIGH, CRITICAL
	Recommendation string  // Diagnóstico o recomendación DBA
}

type FilesystemMount struct {
	MountPoint   string  // /, /u01, /u02, /dev/shm, /tmp, /var
	Device       string  // /dev/mapper/vg_ora-lv_u01, tmpfs...
	FSType       string  // xfs, ext4, tmpfs, nfs, acfs
	TotalMB      int     // Tamaño total en MB
	UsedMB       int     // Espacio usado en MB
	AvailMB      int     // Espacio disponible en MB
	UsedPct      float64 // % uso de bloques
	InodeUsedPct float64 // % uso de inodos (df -i)
	Status       string  // ONLINE, WARNING, CRITICAL
}

type StoragePathInfo struct {
	Name         string // mpatha / ORA_DATA_01 / fc_host0 / sdb / ASM:DATA01
	Type         string // MULTIPATH, FC_HBA, ISCSI, BLOCK_DISK, ORACLEASM
	WWIDOrModel  string // WWID 36005... o Modelo de cabina/disco (PURE FlashArray, VMware Virtual)
	Size         string // 500G, 1.0T, 16 Gbit
	PathsSummary string // 4/4 ACTIVE, 2/4 DEGRADED, Online, running
	PathDetails  string // sdb(OK), sdc(OK), sdd(OK), sde(OK) │ Scheduler: mq-deadline
	Status       string // ONLINE, WARNING, OFFLINE, CRITICAL
}

// SQLPlanInstability represents a SQL_ID with multiple execution plans (Plan Flip / Regression).
type SQLPlanInstability struct {
	SQLID         string
	Schema        string
	PlanCount     int
	Executions    int64
	CurrentPHV    int64
	BestPHV       int64
	AvgElapsedMs  float64
	BestElapsedMs float64
	Adaptive      string // YES, NO
	Status        string // REGRESSION, UNSTABLE, STABLE
	SQLText       string
}

// RMANChannelProgress represents live RMAN backup/restore progress per channel (gv$session_longops + gv$backup_async_io).
type RMANChannelProgress struct {
	InstID      int
	SID         int
	Serial      int
	Channel     string
	Operation   string
	SoFarMB     float64
	TotalMB     float64
	PctComplete float64
	MBPerSec    float64
	ETASec      int
	CurrentFile string
	Status      string // ACTIVE, COMPLETED
}

// SchedulerJobInfo represents business/DBA jobs from dba_scheduler_jobs and dba_jobs.
type SchedulerJobInfo struct {
	JobType      string // SCHEDULER, DBMS_JOB
	Owner        string
	JobName      string
	State        string // SCHEDULED, RUNNING, SUCCEEDED, FAILED, BROKEN, DISABLED
	Enabled      string // TRUE, FALSE
	LastStart    string
	NextRun      string
	RunCount     int
	FailureCount int
	Failed24h    int
	DurationSec  int
	Detail       string
}

// RACCacheFusionStat represents Global Cache (GC) block transfer latency per RAC instance.
type RACCacheFusionStat struct {
	InstID            int
	InstanceName      string
	InterconnectIP    string
	CRBlocksReceived  int64
	CRBlockLatencyMs  float64
	CurBlocksReceived int64
	CurBlockLatencyMs float64
	Status            string // OK, WARNING, CRITICAL
}

// DatafileIOStat represents per-datafile read/write latency from v$filestat.
type DatafileIOStat struct {
	FileID     int
	FileName   string
	Tablespace string
	PhyReads   int64
	PhyWrites  int64
	AvgReadMs  float64
	AvgWriteMs float64
	ReadMB     float64
	WriteMB    float64
	Status     string // OK, SLOW_IO
}

// RedoUndoStat summarizes Redo log switch frequency, Undo health (ORA-01555), and sysmetric rates.
type RedoUndoStat struct {
	RedoSwitchesLastHour int
	RedoSwitches24h      int
	RedoRateMBPerSec     float64
	HardParsesPerSec     float64
	BufferCacheHitPct    float64
	PGACacheHitPct       float64
	UndoActiveMB         float64
	UndoUnexpiredMB      float64
	UndoExpiredMB        float64
	MaxQueryLenSec       int
	ORA01555Count        int
	NoSpaceErrCount      int
	TopIOFunction        string
}

// MemoryAdvisorEntry represents SGA and PGA target advice entries (v$sga_target_advice / v$pga_target_advice).
type MemoryAdvisorEntry struct {
	AdvisorType     string  // SGA, PGA, RESIZE_OP
	SizeMB          int     // Size for estimate
	SizeFactor      float64 // 0.75, 1.0, 1.25, 1.5...
	EstDBTimeFactor float64 // For SGA advice
	EstPhysReads    int64   // For SGA advice
	EstHitPct       float64 // For PGA advice
	OverallocCount  int     // For PGA advice
	Recommendation  string
}

// WaitChainEntry represents multi-hop blocking wait chains (v$wait_chains).
type WaitChainEntry struct {
	ChainID      int
	BlockerInst  int
	BlockerSID   int
	BlockerUser  string
	WaiterInst   int
	WaiterSID    int
	WaiterUser   string
	WaitEvent    string
	WaitSec      int
	LockedObject string
	IsDeadlock   bool
	ChainSummary string // e.g. "INST#2 SID:512 (CRM_ADMIN) ➔ INST#1 SID:289 (CRM_BATCH) ➔ INST#1 SID:415 (ERP_APP)"
}

// ParallelQueryStat represents active Parallel Execution (PX) coordinators and DOP downgrade detection (gv$px_session).
type ParallelQueryStat struct {
	InstID          int
	QCSID           int
	Username        string
	SQLID           string
	ReqDOP          int
	ActualDOP       int
	SlavesAllocated int
	IsDowngraded    bool
	WaitEvent       string
	Status          string // OPTIMAL, DOP_DOWNGRADED
}

// AdvisorFinding represents an automated DBA/SRE diagnostic finding with a 1-click remediation action.
type AdvisorFinding struct {
	Severity     string // CRITICAL, WARNING, INFO
	Category     string // Plan Flip, Cache Fusion, Jobs, I/O & Undo, RMAN, Memory, Locks, SAN
	Title        string
	Detail       string
	Suggestion   string
	Impact       string
	SQLID        string
	ActionID     string
	ActionTarget string
}

type EnvironmentState struct {
	Topology       TopologyMode
	BorderMode     BorderStyle
	HostName       string
	ClusterName    string
	OracleHome     string
	OracleOwner    string
	GridHome       string
	GridOwner      string
	ReleaseVersion string
	IsMock         bool
	LastUpdated    time.Time

	Nodes           []ClusterNode
	Instances       []InstanceInfo
	PDBs            []PDBInfo
	Services        []RACService
	Sessions        []UserSession
	Tablespaces     []TablespaceInfo
	Datafiles       []DatafileInfo
	TableSegments   []TableSegment
	InvalidObjects  []InvalidObject
	Diskgroups      []ASMDiskgroup
	Listeners       []ListenerInfo
	RMANBackups     []RMANBackupJob
	RestorePoints   []RestorePoint
	StandbyDBs      []DataGuardStandby
	AlertEntries    []AlertLogEntry
	AlertTailLines  []string
	DiagTraceDir    string
	AlertFileSizeMB float64
	OSMetrics       []OSHealthMetric
	Filesystems     []FilesystemMount
	StoragePaths    []StoragePathInfo

	// New advanced diagnostics inspired by ora-brabo
	PlanFlips       []SQLPlanInstability
	RMANChannels    []RMANChannelProgress
	SchedulerJobs   []SchedulerJobInfo
	CacheFusion     []RACCacheFusionStat
	DatafileIO      []DatafileIOStat
	RedoUndo        RedoUndoStat
	MemoryAdvisor   []MemoryAdvisorEntry
	WaitChains      []WaitChainEntry
	ParallelQueries []ParallelQueryStat
	AdvisorFindings []AdvisorFinding
}
