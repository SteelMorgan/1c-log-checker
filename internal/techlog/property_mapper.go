package techlog

import (
	"strconv"
	"time"

	"github.com/1c-log-checker/internal/domain"
)

// setRecordProperty sets a property in the record, handling ALL known fields from 1C platform documentation
// Maps property names from tech log to TechLogRecord struct fields
// Note: Some property names are used by multiple event types - we use event name (record.Name) to disambiguate
func setRecordProperty(record *domain.TechLogRecord, key, value string) {
	// Handle ambiguous properties based on event type
	eventName := record.Name
	
	switch key {
	// Core/common fields
	case "level":
		record.Level = value
	case "process":
		record.Process = value
	case "OSThread":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.OSThread = uint32(val)
		}
	case "ClientID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.ClientID = val
		}
	case "SessionID", "Session":
		record.SessionID = value
	case "Trans", "TransactionID":
		record.TransactionID = value
	case "Usr":
		record.User = value
	case "AppID":
		record.ApplicationID = value
	case "ConnID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.ConnectionID = val
		}
	case "Interface":
		record.Interface = value
	case "CallID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.CallID = val
		}
	case "Durationus", "Duration":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Duration = val
			record.DurationUs = val
		}

	// SQL event properties (DBMSSQL, DBPOSTGRS, DBORACLE, DB2, DBV8DBENG, DBDA, EDS)
	case "sql", "Sql":
		record.SQL = value
	case "planSQLText", "PlanSQLText":
		record.PlanSQLText = value
	case "Rows":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Rows = val
		}
	case "RowsAffected":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.RowsAffected = val
		}
	case "Dbms":
		record.DBMS = value
	case "Database":
		record.Database = value
	case "Dbpid":
		record.Dbpid = value
	case "DBCopy":
		record.DBCopy = value
	case "NParams":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.NParams = uint32(val)
		}
	case "MDX":
		record.MDX = value
	case "DBConnID":
		record.DBConnID = value
	case "DBConnStr":
		record.DBConnStr = value
	case "DBUsr":
		record.DBUsr = value

	// SDBL query properties
	case "Query":
		record.Query = value
	case "Sdbl":
		record.Sdbl = value
	case "QueryFileds", "QueryFields":
		record.QueryFields = value

	// Exception properties (EXCP, EXCPCNTX)
	case "Exception":
		record.Exception = value
	case "Descr":
		record.ExceptionDescr = value
	case "Context":
		record.ExceptionContext = value
	case "Func":
		record.Func = value
	case "File":
		// File is used by EXCP (record.File) and FTEXTCHECK (record.FtextFile)
		if eventName == "FTEXTCHECK" || eventName == "FTEXTUPD" {
			record.FtextFile = value
		} else if eventName == "SYSTEM" {
			record.SystemFile = value
		} else {
			record.File = value // EXCP, EXCPCNTX
		}
	case "Module":
		record.Module = value
	case "OSException":
		record.OSException = value

	// Lock properties (TLOCK, TTIMEOUT, TDEADLOCK)
	case "Locks":
		record.Locks = value
	case "Regions":
		record.Regions = value
	case "WaitConnections":
		record.WaitConnections = value
	case "lka":
		record.Lka = value
	case "lkp":
		record.Lkp = value
	case "lkpid":
		record.Lkpid = value
	case "lkaid":
		record.Lkaid = value
	case "lksrc":
		record.Lksrc = value
	case "lkpto":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Lkpto = val
		}
	case "lkato":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Lkato = val
		}
	case "DeadlockConnectionIntersections":
		record.DeadlockConnectionIntersections = value

	// Connection properties (CONN)
	case "Server":
		record.Server = value
	case "Port":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Port = uint32(val)
		}
	case "SyncPort":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SyncPort = uint32(val)
		}
	case "hResultOLEDB":
		record.HResultOLEDB = value
	case "hResultNC2005":
		record.HResultNC2005 = value
	case "hResultNC2008":
		record.HResultNC2008 = value
	case "hResult2012", "hResultNC2012":
		record.HResultNC2012 = value

	// Session properties (SESN)
	case "Nmb":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SessionNmb = val
		}
	case "seanceID":
		record.SeanceID = value

	// Process properties (PROC)
	case "ProcID":
		record.ProcID = value
	case "PID":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.PID = uint32(val)
		}
	case "ProcessName":
		record.ProcessName = value
	case "p:processName":
		record.PProcessName = value
	case "srcProcessName":
		record.SrcProcessName = value
	case "Finish":
		record.Finish = value
	case "ExitCode":
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			record.ExitCode = int32(val)
		}
	case "RunAs":
		record.RunAs = value

	// Call properties (CALL, SCALL)
	case "MName":
		record.MName = value
	case "IName":
		record.IName = value
	case "DstClientID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.DstClientID = val
		}
	case "RetExcp":
		record.RetExcp = value
	case "Memory":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Memory = val
		}
	case "MemoryPeak":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.MemoryPeak = val
		}

	// Cluster properties (CLSTR) - основные
	case "Event":
		record.ClusterEvent = value
	case "Cluster":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Cluster = uint32(val)
		}
	case "IB":
		record.IB = value
	case "Ref":
		record.Ref = value
	case "Connections":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Connections = uint32(val)
		}
	case "ConnLimit":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.ConnLimit = uint32(val)
		}
	case "Infobases":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Infobases = uint32(val)
		}
	case "IBLimit":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.IBLimit = uint32(val)
		}
	case "DstAddr":
		record.DstAddr = value
	case "DstId":
		record.DstId = value
	case "DstPid":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.DstPid = uint32(val)
		}
	case "DstSrv":
		record.DstSrv = value
	case "SrcAddr":
		record.SrcAddr = value
	case "SrcId":
		record.SrcId = value
	case "SrcPid":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SrcPid = uint32(val)
		}
	case "SrcSrv":
		record.SrcSrv = value
	case "SrcURL":
		record.SrcURL = value
	case "MyVer":
		record.MyVer = value
	case "SrcVer":
		record.SrcVer = value
	case "Registered":
		record.Registered = value
	case "Obsolete":
		record.Obsolete = value
	case "Released":
		record.Released = value
	case "Reason":
		record.Reason = value
	case "Request":
		record.Request = value
	case "ServiceName":
		record.ServiceName = value
	case "ApplicationExt":
		record.ApplicationExt = value
	case "NeedResync":
		record.NeedResync = value
	case "NewServiceDataDirectory":
		record.NewServiceDataDirectory = value
	case "OldServiceDataDirectory":
		record.OldServiceDataDirectory = value

	// Server context properties (SCOM)
	case "ServerComputerName":
		record.ServerComputerName = value
	case "procURL":
		record.ProcURL = value
	case "agentURL":
		record.AgentURL = value

	// Admin properties (ADMIN)
	case "Admin":
		record.Admin = value
	case "Action":
		record.Action = value

	// Memory properties (MEM, LEAKS, ATTN)
	case "Sz":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.Sz = val
		}
	case "Szd":
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			record.Szd = val
		}
	case "cn":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Cn = uint32(val)
		}
	case "cnd":
		if val, err := strconv.ParseInt(value, 10, 32); err == nil {
			record.Cnd = int32(val)
		}
	case "MemoryLimits":
		record.MemoryLimits = value
	case "ExcessDurationSec":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.ExcessDurationSec = val
		}
	case "ExcessStartTime":
		if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
			record.ExcessStartTime = t
		}
	case "FreeMemory":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FreeMemory = val
		}
	case "TotalMemory":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.TotalMemory = val
		}
	case "SafeLimit":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SafeLimit = val
		}
	case "Info":
		// Info is used by ATTN (record.AttnInfo) and FTEXTCHECK (record.FtextInfo)
		if eventName == "FTEXTCHECK" || eventName == "FTEXTUPD" {
			record.FtextInfo = value
		} else {
			record.AttnInfo = value // ATTN
		}
	case "ProcessId":
		record.AttnProcessID = value
	case "ServerId":
		record.AttnServerID = value
	case "Url":
		record.AttnURL = value

	// License properties (LIC, HASP)
	case "res":
		record.LicRes = value
	case "HaspID":
		record.HaspID = value

	// Full-text search properties (FTEXTUPD, FTS, FTEXTCHECK, INPUTBYSTRING)
	case "State":
		record.FtextState = value
	case "AvMem":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.AvMem = val
		}
	case "BackgroundJobCreated":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.BackgroundJobCreated = 1
			}
		}
	case "MemoryUsed":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.MemoryUsed = val
		}
	case "FailedJobsCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FailedJobsCount = uint32(val)
		}
	case "TotalJobsCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.TotalJobsCount = uint32(val)
		}
	case "JobCanceledByLoadLimit":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.JobCanceledByLoadLimit = 1
			}
		}
	case "MinDataId":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.MinDataID = val
		}
	case "Files":
		record.FtextFiles = value
	case "FilesCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtextFilesCount = uint32(val)
		}
	case "FilesTotalSize":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FtextFilesTotalSize = val
		}
	case "Folder":
		record.FtextFolder = value
	case "Time":
		record.FtextTime = value
	case "Result":
		// Result is used by FTEXTCHECK (record.FtextResult) and ADDIN (record.AddinResult)
		if eventName == "ADDIN" {
			if val, err := strconv.ParseUint(value, 10, 8); err == nil {
				record.AddinResult = uint8(val)
			}
		} else if eventName == "FTEXTCHECK" || eventName == "FTEXTUPD" {
			if val, err := strconv.ParseUint(value, 10, 8); err == nil {
				record.FtextResult = uint8(val)
			}
		}
	case "Separation":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.FtextSeparation = 1
			}
		}
	case "SepId":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtextSepID = uint32(val)
		}
	case "Word":
		record.FtextWord = value
	case "FindByString":
		record.FindByString = value
	case "Text":
		record.InputText = value
	case "findTicks":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FindTicks = val
		}
	case "ftextTicks":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FtextTicks = val
		}
	case "ftextSearchCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtextSearchCount = uint32(val)
		}
	case "ftextResultCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtextResultCount = uint32(val)
		}
	case "SearchByMask":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.SearchByMask = 1
			}
		}
	case "tooManyResults":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.TooManyResults = 1
			}
		}
	case "FillRefsPresent":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.FillRefsPresent = 1
			}
		}
	case "jobId":
		record.FtsJobID = value
	case "logFrom":
		record.FtsLogFrom = value
	case "logTo":
		record.FtsLogTo = value
	case "fixedState":
		record.FtsFixedState = value
	case "recordCount":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FtsRecordCount = val
		}
	case "totalRecords":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FtsTotalRecords = val
		}
	case "tableCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtsTableCount = uint32(val)
		}
	case "tableName":
		record.FtsTableName = value
	case "tableCode":
		record.FtsTableCode = value
	case "tableRef":
		record.FtsTableRef = value
	case "metaDataId":
		record.FtsMetadataID = value
	case "recordRef":
		record.FtsRecordRef = value
	case "fullKey":
		record.FtsFullKey = value
	case "reindexCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtsReindexCount = uint32(val)
		}
	case "skippedRecords":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.FtsSkippedRecords = val
		}
	case "parallelism":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.FtsParallelism = uint32(val)
		}

	// Storage properties (STORE)
	case "Id":
		// Id is used by STORE (record.StoreID) and STT/STTAdm (record.SttID)
		if eventName == "STT" || eventName == "STTAdm" {
			record.SttID = value
		} else if eventName == "STORE" {
			record.StoreID = value
		} else {
			// Default to StoreID for backward compatibility
			record.StoreID = value
		}
	case "Size":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.StoreSize = val
		}
	case "StorageGUID":
		record.StorageGUID = value
	case "BackupFileName":
		record.BackupFileName = value
	case "BackupBaseFileName":
		record.BackupBaseFileName = value
	case "BackupType":
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			record.BackupType = uint8(val)
		}
	case "MinimalWriteSize":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.MinimalWriteSize = val
		}
	case "ReadOnlyMode":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.ReadOnlyMode = 1
			}
		}
	case "UseMode":
		record.UseMode = value

	// Garbage collector properties (SDGC)
	case "InstanceID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SDGCInstanceID = val
		}
	case "Method":
		// Method is used by CALL/SCALL (record.Method), SDGC (record.SDGCMethod), and VRSREQUEST/VRSRESPONSE (record.VrsMethod)
		if eventName == "SDGC" {
			record.SDGCMethod = value
		} else if eventName == "VRSREQUEST" || eventName == "VRSRESPONSE" {
			record.VrsMethod = value
		} else if eventName == "MAILPARSEERR" {
			record.MailMethod = value
		} else {
			record.Method = value // CALL, SCALL
		}
	case "FilesSize":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SDGCFilesSize = val
		}
	case "UsedSize":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SDGCUsedSize = val
		}
	case "CopyBytes":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SDGCCopyBytes = val
		}
	case "LockDuration":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.SDGCLockDuration = val
		}

	// Add-in properties (ADDIN)
	case "Classes":
		record.AddinClasses = value
	case "Location":
		record.AddinLocation = value
	case "MethodName":
		record.AddinMethodName = value
	case "Message":
		record.AddinMessage = value
	case "Source":
		record.AddinSource = value
	case "Type":
		record.AddinType = value
	case "Crashed":
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			record.AddinCrashed = uint8(val)
		}
	case "ErrorDescr":
		record.AddinErrorDescr = value

	// System event properties (SYSTEM)
	case "Class":
		record.SystemClass = value
	case "Component":
		record.SystemComponent = value
	case "Line":
		// Line is used by EXCP (record.Line) and SYSTEM (record.SystemLine)
		if eventName == "SYSTEM" {
			if val, err := strconv.ParseUint(value, 10, 32); err == nil {
				record.SystemLine = uint32(val)
			}
		} else {
			// EXCP, EXCPCNTX
			if val, err := strconv.ParseUint(value, 10, 32); err == nil {
				record.Line = uint32(val)
			}
		}
	case "Txt":
		record.SystemTxt = value

	// Event log properties (EVENTLOG)
	case "FileName":
		record.EventlogFileName = value
	case "CpuTime":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.EventlogCPUTime = val
		}
	case "PacketCount":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.EventlogPacketCount = uint32(val)
		}

	// Video properties (VIDEOCALL, VIDEOCONN, VIDEOSTATS)
	case "Connection":
		// Connection is used by CONN (record.Connection) and VIDEOCONN (record.VideoConnection)
		if eventName == "VIDEOCONN" || eventName == "VIDEOCALL" || eventName == "VIDEOSTATS" {
			record.VideoConnection = value
		} else {
			// CONN
			if val, err := strconv.ParseUint(value, 10, 64); err == nil {
				record.Connection = val
			}
		}
	case "Status":
		// Status is used by VIDEOCONN (record.VideoStatus), VRSRESPONSE (record.VrsStatus), and STT (record.SttStatus)
		if eventName == "VIDEOCONN" || eventName == "VIDEOCALL" || eventName == "VIDEOSTATS" {
			record.VideoStatus = value
		} else if eventName == "VRSRESPONSE" {
			if val, err := strconv.ParseUint(value, 10, 32); err == nil {
				record.VrsStatus = uint32(val)
			}
		} else if eventName == "STT" || eventName == "STTAdm" {
			record.SttStatus = value
		}
	case "StreamType":
		record.VideoStreamType = value
	case "Value":
		record.VideoValue = value
	case "cpu":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.VideoCPU = uint32(val)
		}
	case "QueueLenght":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.VideoQueueLength = uint32(val)
		}
	case "InMessage":
		record.VideoInMessage = value
	case "OutMessage":
		record.VideoOutMessage = value
	case "Direction":
		record.VideoDirection = value

	// Speech recognition properties (STT, STTAdm)
	case "Key":
		record.SttKey = value
	case "modelID":
		record.SttModelID = value
	case "Path":
		record.SttPath = value
	case "AudioEncoding":
		record.SttAudioEncoding = value
	case "Frames":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SttFrames = uint32(val)
		}
	case "Contexts":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SttContexts = uint32(val)
		}
	case "ContextsOnly":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.SttContextsOnly = 1
			}
		}
	case "Recording":
		if val, err := strconv.ParseBool(value); err == nil {
			if val {
				record.SttRecording = 1
			}
		}
	case "rx:Acoustic":
		record.SttRxAcoustic = value
	case "rx:Grammar":
		record.SttRxGrammar = value
	case "rx:Language":
		record.SttRxLanguage = value
	case "rx:Location":
		record.SttRxLocation = value
	case "rx:SampleRate":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SttRxSampleRate = uint32(val)
		}
	case "rx:Version":
		record.SttRxVersion = value
	case "tx:Acoustic":
		record.SttTxAcoustic = value
	case "tx:Grammar":
		record.SttTxGrammar = value
	case "tx:Language":
		record.SttTxLanguage = value
	case "tx:Location":
		record.SttTxLocation = value
	case "tx:SampleRate":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.SttTxSampleRate = uint32(val)
		}
	case "tx:Version":
		record.SttTxVersion = value

	// Web service properties (VRSREQUEST, VRSRESPONSE)
	case "URI":
		record.VrsURI = value
	case "Headers":
		record.VrsHeaders = value
	case "Body":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.VrsBody = val
		}
	case "Phrase":
		// Phrase is used by STT/STTAdm (record.SttPhrase) and VRSRESPONSE (record.VrsPhrase)
		if eventName == "VRSRESPONSE" {
			record.VrsPhrase = value
		} else {
			record.SttPhrase = value // STT, STTAdm
		}

	// Integration properties (SINTEG, EDS)
	case "SrvcName":
		record.SintegSrvcName = value
	case "ExtSrvcUrl":
		record.SintegExtSrvcURL = value
	case "ExtSrvcUsr":
		record.SintegExtSrvcUsr = value

	// Mail properties (MAILPARSEERR)
	case "MessageUid":
		record.MailMessageUID = value

	// Certificate properties (WINCERT)
	case "certificate":
		record.WinCertCertificate = value
	case "errorCode":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.WinCertErrorCode = uint32(val)
		}

	// Data history properties (DHIST)
	case "description":
		record.DhistDescription = value

	// Config load properties (CONFLOADFROMFILES)
	// Action already handled above

	// Background job properties
	case "Report":
		record.Report = value

	// Client properties (t: prefix)
	case "t:applicationName":
		record.TApplicationName = value
	case "t:clientID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.TClientID = val
		}
	case "t:computerName":
		record.TComputerName = value
	case "t:connectID":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.TConnectID = val
		}

	// Additional properties
	case "Host":
		record.Host = value
	case "Val":
		record.Val = value
	case "Err":
		if val, err := strconv.ParseUint(value, 10, 8); err == nil {
			record.Err = uint8(val)
		}
	case "Calls":
		if val, err := strconv.ParseUint(value, 10, 32); err == nil {
			record.Calls = uint32(val)
		}
	case "InBytes":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.InBytes = val
		}
	case "OutBytes":
		if val, err := strconv.ParseUint(value, 10, 64); err == nil {
			record.OutBytes = val
		}

	default:
		// Store in dynamic properties
		record.Properties[key] = value
	}
}

