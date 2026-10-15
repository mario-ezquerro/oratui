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

	Nodes          []ClusterNode
	Instances      []InstanceInfo
	PDBs           []PDBInfo
	Services       []RACService
	Sessions       []UserSession
	Tablespaces    []TablespaceInfo
	Datafiles      []DatafileInfo
	TableSegments  []TableSegment
	InvalidObjects []InvalidObject
	Diskgroups     []ASMDiskgroup
	Listeners      []ListenerInfo
	RMANBackups    []RMANBackupJob
	RestorePoints  []RestorePoint
	StandbyDBs      []DataGuardStandby
	AlertEntries    []AlertLogEntry
	AlertTailLines  []string
	DiagTraceDir    string
	AlertFileSizeMB float64
}
