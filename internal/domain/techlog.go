package domain

import "time"

// TechLogRecord represents a single record from 1C Tech Log (Технологический журнал)
// Contains ALL possible fields from 1C platform documentation (ITS 8.3.27, section 3.24.2.4.2)
// See internal/domain/techlog_full.go for the complete structure with all fields
// This file contains a simplified version for backward compatibility during migration
type TechLogRecord struct {
	// Core fields (present in all events)
	Timestamp    time.Time
	Duration     uint64 // microseconds (durationus)
	Name         string // Event name (PROC, CONN, DBMSSQL, etc.)
	Level        string // INFO, WARN, ERROR
	Depth        uint8  // Nesting level in call stack
	Process      string // Process name
	OSThread     uint32 // OS thread ID
	
	// Common fields (may be absent in some events)
	ClientID      uint64
	SessionID     string
	TransactionID string
	User          string
	ApplicationID string
	ConnectionID  uint64
	Interface     string
	Method        string
	CallID        uint64
	
	// Cluster/Infobase identification
	ClusterGUID   string
	InfobaseGUID  string
	
	// Raw line for forensics
	RawLine string
	
	// SQL event properties (DBMSSQL, DBPOSTGRS, DBORACLE, DB2, DBV8DBENG, DBDA, EDS)
	SQL          string
	PlanSQLText  string
	Rows         uint64
	RowsAffected uint64
	DBMS         string
	Database     string
	Dbpid        string
	DBCopy       string
	NParams      uint32
	MDX          string
	DBConnID     string
	DBConnStr    string
	DBUsr        string
	
	// SDBL query properties
	Query        string
	Sdbl         string
	QueryFields  string
	
	// Exception properties (EXCP, EXCPCNTX)
	Exception        string
	ExceptionDescr   string
	ExceptionContext string
	Func             string
	Line             uint32
	File             string
	Module           string
	OSException      string
	
	// Lock properties (TLOCK, TTIMEOUT, TDEADLOCK)
	Locks                        string
	Regions                      string
	WaitConnections              string
	Lka                          string
	Lkp                          string
	Lkpid                        string
	Lkaid                        string
	Lksrc                        string
	Lkpto                        uint64
	Lkato                        uint64
	DeadlockConnectionIntersections string
	
	// Connection properties (CONN)
	Server        string
	Port          uint32
	SyncPort      uint32
	Connection    uint64
	HResultOLEDB  string
	HResultNC2005 string
	HResultNC2008 string
	HResultNC2012 string
	
	// Session properties (SESN)
	SessionNmb uint64
	SeanceID   string
	
	// Process properties (PROC)
	ProcID          string
	PID             uint32
	ProcessName     string
	PProcessName    string
	SrcProcessName  string
	Finish          string
	ExitCode        int32
	RunAs           string
	
	// Call properties (CALL, SCALL)
	MName        string
	IName        string
	DstClientID  uint64
	RetExcp      string
	Memory       uint64
	MemoryPeak   uint64
	
	// Cluster properties (CLSTR) - основные, остальные в Properties
	ClusterEvent              string
	Cluster                   uint32
	IB                        string
	Ref                       string
	Connections               uint32
	ConnLimit                 uint32
	Infobases                 uint32
	IBLimit                   uint32
	DstAddr                   string
	DstId                     string
	DstPid                    uint32
	DstSrv                    string
	SrcAddr                   string
	SrcId                     string
	SrcPid                    uint32
	SrcSrv                    string
	SrcURL                    string
	MyVer                     string
	SrcVer                    string
	Registered                string
	Obsolete                  string
	Released                  string
	Reason                    string
	Request                   string
	ServiceName               string
	ApplicationExt            string
	NeedResync                string
	NewServiceDataDirectory   string
	OldServiceDataDirectory   string
	
	// Server context properties (SCOM)
	ServerComputerName string
	ProcURL            string
	AgentURL           string
	
	// Admin properties (ADMIN)
	Admin  string
	Action string
	
	// Memory properties (MEM, LEAKS, ATTN)
	Sz                uint64
	Szd               int64
	Cn                uint32
	Cnd               int32
	MemoryLimits      string
	ExcessDurationSec uint64
	ExcessStartTime   time.Time
	FreeMemory        uint64
	TotalMemory       uint64
	SafeLimit         uint64
	AttnInfo          string
	AttnPID           uint32
	AttnProcessID     string
	AttnServerID      string
	AttnURL           string
	
	// License properties (LIC, HASP)
	LicRes string
	HaspID string
	
	// Full-text search properties (FTEXTUPD, FTS, FTEXTCHECK, INPUTBYSTRING)
	FtextState                string
	AvMem                     uint64
	BackgroundJobCreated      uint8
	MemoryUsed                uint64
	FailedJobsCount           uint32
	TotalJobsCount            uint32
	JobCanceledByLoadLimit    uint8
	MinDataID                 uint64
	FtextFiles                string
	FtextFilesCount           uint32
	FtextFilesTotalSize       uint64
	FtextFolder               string
	FtextTime                 string
	FtextFile                 string
	FtextInfo                 string
	FtextResult               uint8
	FtextSeparation           uint8
	FtextSepID                uint32
	FtextWord                 string
	FindByString              string
	InputText                 string
	FindTicks                 uint64
	FtextTicks                uint64
	FtextSearchCount          uint32
	FtextResultCount          uint32
	SearchByMask              uint8
	TooManyResults            uint8
	FillRefsPresent           uint8
	FtsJobID                  string
	FtsLogFrom                string
	FtsLogTo                  string
	FtsFixedState             string
	FtsRecordCount            uint64
	FtsTotalRecords           uint64
	FtsTableCount             uint32
	FtsTableName              string
	FtsTableCode              string
	FtsTableRef               string
	FtsMetadataID             string
	FtsRecordRef              string
	FtsFullKey                string
	FtsReindexCount           uint32
	FtsSkippedRecords         uint64
	FtsParallelism            uint32
	
	// Storage properties (STORE)
	StoreID              string
	StoreSize            uint64
	StorageGUID          string
	BackupFileName       string
	BackupBaseFileName   string
	BackupType           uint8
	MinimalWriteSize     uint64
	ReadOnlyMode         uint8
	UseMode              string
	
	// Garbage collector properties (SDGC)
	SDGCInstanceID   uint64
	SDGCMethod       string
	SDGCFilesSize    uint64
	SDGCUsedSize     uint64
	SDGCCopyBytes    uint64
	SDGCLockDuration uint64
	
	// Add-in properties (ADDIN)
	AddinClasses     string
	AddinLocation    string
	AddinMethodName  string
	AddinMessage     string
	AddinSource      string
	AddinType        string
	AddinResult      uint8
	AddinCrashed     uint8
	AddinErrorDescr  string
	
	// System event properties (SYSTEM)
	SystemClass      string
	SystemComponent  string
	SystemFile       string
	SystemLine       uint32
	SystemTxt        string
	
	// Event log properties (EVENTLOG)
	EventlogFileName    string
	EventlogCPUTime     uint64
	EventlogOSThread    uint32
	EventlogPacketCount uint32
	
	// Video properties (VIDEOCALL, VIDEOCONN, VIDEOSTATS)
	VideoConnection  string
	VideoStatus      string
	VideoStreamType  string
	VideoValue       string
	VideoCPU         uint32
	VideoQueueLength uint32
	VideoInMessage   string
	VideoOutMessage  string
	VideoDirection   string
	VideoType        string
	
	// Speech recognition properties (STT, STTAdm)
	SttID            string
	SttKey           string
	SttModelID       string
	SttPath          string
	SttAudioEncoding string
	SttFrames        uint32
	SttContexts      uint32
	SttContextsOnly  uint8
	SttRecording     uint8
	SttStatus        string
	SttPhrase        string
	SttRxAcoustic    string
	SttRxGrammar     string
	SttRxLanguage    string
	SttRxLocation    string
	SttRxSampleRate  uint32
	SttRxVersion     string
	SttTxAcoustic    string
	SttTxGrammar     string
	SttTxLanguage    string
	SttTxLocation    string
	SttTxSampleRate  uint32
	SttTxVersion     string
	
	// Web service properties (VRSREQUEST, VRSRESPONSE)
	VrsURI     string
	VrsMethod  string
	VrsHeaders string
	VrsBody    uint64
	VrsStatus  uint32
	VrsPhrase  string
	
	// Integration properties (SINTEG, EDS)
	SintegSrvcName    string
	SintegExtSrvcURL  string
	SintegExtSrvcUsr  string
	
	// Mail properties (MAILPARSEERR)
	MailMessageUID string
	MailMethod     string
	
	// Certificate properties (WINCERT)
	WinCertCertificate string
	WinCertErrorCode   uint32
	
	// Data history properties (DHIST)
	DhistDescription string
	
	// Config load properties (CONFLOADFROMFILES)
	ConfLoadAction string
	
	// Background job properties
	Report string
	
	// Client properties (t: prefix)
	TApplicationName string
	TClientID        uint64
	TComputerName    string
	TConnectID       uint64
	
	// Additional properties
	Host      string
	Val       string
	Err       uint8
	Calls     uint32
	InBytes   uint64
	OutBytes  uint64
	DurationUs uint64 // Durationus (alternative to Duration)
	
	// Dynamic properties (all other event-specific fields)
	// Stored as key-value map to handle arbitrary tech log properties
	Properties map[string]string
}

