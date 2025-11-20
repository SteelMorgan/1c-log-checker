-- Tech Log Table (Технологический журнал)
CREATE TABLE IF NOT EXISTS logs.tech_log (
    ts DateTime64(6) CODEC(Delta, ZSTD),
    duration UInt64 CODEC(T64, ZSTD),
    name LowCardinality(String) CODEC(ZSTD),
    level LowCardinality(String) CODEC(ZSTD),
    depth UInt8 CODEC(T64, ZSTD),
    process LowCardinality(String) CODEC(ZSTD),
    os_thread UInt32 CODEC(T64, ZSTD),
    client_id UInt64 CODEC(T64, ZSTD),
    session_id String CODEC(ZSTD),
    transaction_id String CODEC(ZSTD),
    usr String CODEC(ZSTD),
    app_id String CODEC(ZSTD),
    connection_id UInt64 CODEC(T64, ZSTD),
    interface String CODEC(ZSTD),
    method String CODEC(ZSTD),
    call_id UInt64 CODEC(T64, ZSTD),
    -- Cluster/Infobase identification (extracted from log path or config)
    cluster_guid String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    -- Raw log line for forensics
    raw_line String CODEC(ZSTD),
    
    -- SQL event properties (DBMSSQL, DBPOSTGRS, DBORACLE, DB2, DBV8DBENG, DBDA, EDS)
    sql String CODEC(ZSTD),                       -- SQL query text
    plan_sql_text String CODEC(ZSTD),             -- Query execution plan
    rows UInt64 CODEC(T64, ZSTD),                 -- Records returned
    rows_affected UInt64 CODEC(T64, ZSTD),        -- Records modified
    dbms LowCardinality(String) CODEC(ZSTD),      -- DBMS type (DBMSSQL, DBPOSTGRS, etc.)
    database String CODEC(ZSTD),                  -- Database name
    dbpid String CODEC(ZSTD),                     -- DB process ID (for DBMSSQL, DBPOSTGRS, DB2, DBORACLE)
    db_copy String CODEC(ZSTD),                   -- Database copy name
    n_params UInt32 CODEC(T64, ZSTD),             -- Number of SQL parameters (for DBV8DBENG)
    mdx String CODEC(ZSTD),                       -- MDX query text (for EDS/OLAP)
    db_conn_id String CODEC(ZSTD),                -- DB connection ID (for EDS)
    db_conn_str String CODEC(ZSTD),               -- DB connection string (for EDS)
    db_usr String CODEC(ZSTD),                    -- DB user name (for EDS)
    
    -- SDBL query properties
    query String CODEC(ZSTD),                     -- Query text (for SDBL, QERR)
    sdbl String CODEC(ZSTD),                      -- SDBL query text
    query_fields String CODEC(ZSTD),              -- Query fields with NULL (for QERR)
    
    -- Exception properties (EXCP, EXCPCNTX)
    exception String CODEC(ZSTD),                 -- Exception name
    exception_descr String CODEC(ZSTD),           -- Exception description
    exception_context String CODEC(ZSTD),         -- Execution context (call stack)
    func String CODEC(ZSTD),                      -- Function/method name
    line UInt32 CODEC(T64, ZSTD),                 -- Line number
    file String CODEC(ZSTD),                      -- File name
    module String CODEC(ZSTD),                    -- Module name
    os_exception String CODEC(ZSTD),              -- OS exception description
    
    -- Lock properties (TLOCK, TTIMEOUT, TDEADLOCK)
    locks String CODEC(ZSTD),                     -- List of managed locks
    regions String CODEC(ZSTD),                    -- Lock region names
    wait_connections String CODEC(ZSTD),          -- Connections being blocked
    lka String CODEC(ZSTD),                       -- Lock source (aggressor)
    lkp String CODEC(ZSTD),                       -- Lock victim
    lkpid String CODEC(ZSTD),                     -- Lock victim request ID
    lkaid String CODEC(ZSTD),                     -- Lock aggressor request IDs
    lksrc String CODEC(ZSTD),                     -- Lock source connection ID
    lkpto UInt64 CODEC(T64, ZSTD),                -- Lock victim timeout
    lkato UInt64 CODEC(T64, ZSTD),                -- Lock aggressor timeout
    deadlock_connection_intersections String CODEC(ZSTD), -- Deadlock transaction pairs (for TDEADLOCK)
    
    -- Connection properties (CONN)
    server String CODEC(ZSTD),                    -- Server name
    port UInt32 CODEC(T64, ZSTD),                 -- Main network port
    sync_port UInt32 CODEC(T64, ZSTD),            -- Sync network port
    connection UInt64 CODEC(T64, ZSTD),           -- Connection number
    h_result_oledb String CODEC(ZSTD),            -- OLE DB provider return code
    h_result_nc2005 String CODEC(ZSTD),           -- SQL Server Native Client 2005 return code
    h_result_nc2008 String CODEC(ZSTD),           -- SQL Server Native Client 2008 return code
    h_result_nc2012 String CODEC(ZSTD),           -- SQL Server Native Client 2012 return code
    
    -- Session properties (SESN)
    session_nmb UInt64 CODEC(T64, ZSTD),          -- Session number (Nmb)
    seance_id String CODEC(ZSTD),                 -- Session identifier
    
    -- Process properties (PROC)
    proc_id String CODEC(ZSTD),                   -- Process identifier
    pid UInt32 CODEC(T64, ZSTD),                  -- OS process ID
    process_name String CODEC(ZSTD),               -- Process name
    p_process_name String CODEC(ZSTD),            -- Server context name (p:processName)
    src_process_name String CODEC(ZSTD),           -- Source process name
    finish String CODEC(ZSTD),                    -- Process finish reason
    exit_code Int32 CODEC(T64, ZSTD),             -- Process exit code
    run_as String CODEC(ZSTD),                    -- Run mode (application/service)
    
    -- Call properties (CALL, SCALL)
    m_name String CODEC(ZSTD),                   -- Method name (MName)
    i_name String CODEC(ZSTD),                   -- Interface name (IName)
    dst_client_id UInt64 CODEC(T64, ZSTD),        -- Destination client ID
    ret_excp String CODEC(ZSTD),                  -- Returned exception
    memory UInt64 CODEC(T64, ZSTD),               -- Memory used (bytes)
    memory_peak UInt64 CODEC(T64, ZSTD),          -- Peak memory used (bytes)
    
    -- Cluster properties (CLSTR)
    cluster_event String CODEC(ZSTD),              -- Cluster event type (Event)
    cluster UInt32 CODEC(T64, ZSTD),              -- Cluster port number
    ib String CODEC(ZSTD),                        -- Information base name
    ref String CODEC(ZSTD),                       -- Information base name (Ref)
    connections UInt32 CODEC(T64, ZSTD),          -- Connections count
    conn_limit UInt32 CODEC(T64, ZSTD),           -- Connection limit per process
    infobases UInt32 CODEC(T64, ZSTD),            -- Infobases count
    ib_limit UInt32 CODEC(T64, ZSTD),             -- Infobase limit per process
    dst_addr String CODEC(ZSTD),                  -- Destination process address
    dst_id String CODEC(ZSTD),                    -- Destination process ID
    dst_pid UInt32 CODEC(T64, ZSTD),              -- Destination process PID
    dst_srv String CODEC(ZSTD),                  -- Destination server name
    src_addr String CODEC(ZSTD),                  -- Source process address
    src_id String CODEC(ZSTD),                    -- Source process ID
    src_pid UInt32 CODEC(T64, ZSTD),              -- Source process PID
    src_srv String CODEC(ZSTD),                  -- Source server name
    src_url String CODEC(ZSTD),                   -- Source server URL
    my_ver String CODEC(ZSTD),                    -- Current server version
    src_ver String CODEC(ZSTD),                   -- Received cluster version
    registered String CODEC(ZSTD),                -- Registered processes
    obsolete String CODEC(ZSTD),                  -- Obsolete processes
    released String CODEC(ZSTD),                  -- Released processes
    reason String CODEC(ZSTD),                    -- Process unavailability reason
    request String CODEC(ZSTD),                   -- Connection request ID
    service_name String CODEC(ZSTD),              -- Service name
    application_ext String CODEC(ZSTD),           -- Application extension
    need_resync String CODEC(ZSTD),              -- Need resync flag
    new_service_data_directory String CODEC(ZSTD), -- New service data directory
    old_service_data_directory String CODEC(ZSTD), -- Old service data directory
    
    -- Server context properties (SCOM)
    server_computer_name String CODEC(ZSTD),      -- Server computer name
    proc_url String CODEC(ZSTD),                  -- Process server URL
    agent_url String CODEC(ZSTD),                 -- Agent server URL
    
    -- Admin properties (ADMIN)
    admin String CODEC(ZSTD),                     -- Administrator name
    action String CODEC(ZSTD),                    -- Action description
    
    -- Memory properties (MEM, LEAKS, ATTN)
    sz UInt64 CODEC(T64, ZSTD),                   -- Memory size (bytes)
    szd Int64 CODEC(T64, ZSTD),                   -- Memory size delta (bytes)
    cn UInt32 CODEC(T64, ZSTD),                   -- Memory chunks count
    cnd Int32 CODEC(T64, ZSTD),                   -- Memory chunks delta
    memory_limits String CODEC(ZSTD),              -- Memory limits (for ATTN)
    excess_duration_sec UInt64 CODEC(T64, ZSTD),   -- Excess duration seconds (for ATTN)
    excess_start_time DateTime64(6) CODEC(Delta, ZSTD), -- Excess start time (for ATTN)
    free_memory UInt64 CODEC(T64, ZSTD),          -- Free memory (for ATTN)
    total_memory UInt64 CODEC(T64, ZSTD),          -- Total memory (for ATTN)
    safe_limit UInt64 CODEC(T64, ZSTD),           -- Safe memory limit (for ATTN)
    attn_info String CODEC(ZSTD),                 -- ATTN info (connection attempts)
    attn_pid UInt32 CODEC(T64, ZSTD),              -- ATTN process ID
    attn_process_id String CODEC(ZSTD),           -- ATTN process identifier
    attn_server_id String CODEC(ZSTD),            -- ATTN server identifier
    attn_url String CODEC(ZSTD),                   -- ATTN manager URL
    
    -- License properties (LIC, HASP)
    lic_res String CODEC(ZSTD),                   -- License action result
    hasp_id String CODEC(ZSTD),                   -- HASP key ID
    
    -- Full-text search properties (FTEXTUPD, FTS, FTEXTCHECK, INPUTBYSTRING)
    ftext_state String CODEC(ZSTD),               -- FTS state (for FTEXTUPD)
    av_mem UInt64 CODEC(T64, ZSTD),               -- Available memory (for FTEXTUPD)
    background_job_created UInt8 CODEC(T64, ZSTD), -- Background job created (for FTEXTUPD)
    memory_used UInt64 CODEC(T64, ZSTD),          -- Memory used (for FTEXTUPD)
    failed_jobs_count UInt32 CODEC(T64, ZSTD),    -- Failed jobs count (for FTEXTUPD)
    total_jobs_count UInt32 CODEC(T64, ZSTD),     -- Total jobs count (for FTEXTUPD)
    job_canceled_by_load_limit UInt8 CODEC(T64, ZSTD), -- Job canceled flag (for FTEXTUPD)
    min_data_id UInt64 CODEC(T64, ZSTD),          -- Min data ID (for FTEXTUPD)
    ftext_files String CODEC(ZSTD),               -- Files list (for FTEXTUPD)
    ftext_files_count UInt32 CODEC(T64, ZSTD),     -- Files count (for FTEXTUPD)
    ftext_files_total_size UInt64 CODEC(T64, ZSTD), -- Files total size (for FTEXTUPD)
    ftext_folder String CODEC(ZSTD),               -- Folder path (for FTEXTUPD)
    ftext_time String CODEC(ZSTD),                -- Time string (for FTEXTUPD/FTS)
    ftext_file String CODEC(ZSTD),                -- File name (for FTEXTCHECK)
    ftext_info String CODEC(ZSTD),                -- Info text (for FTEXTCHECK)
    ftext_result UInt8 CODEC(T64, ZSTD),          -- Result (for FTEXTCHECK)
    ftext_separation UInt8 CODEC(T64, ZSTD),      -- Separation enabled (for FTEXTCHECK)
    ftext_sep_id UInt32 CODEC(T64, ZSTD),         -- Separation ID (for FTEXTCHECK)
    ftext_word String CODEC(ZSTD),                -- Word (for FTEXTCHECK)
    find_by_string String CODEC(ZSTD),            -- Object name (for INPUTBYSTRING)
    input_text String CODEC(ZSTD),                -- Input text (for INPUTBYSTRING)
    find_ticks UInt64 CODEC(T64, ZSTD),           -- Find time ms (for INPUTBYSTRING)
    ftext_ticks UInt64 CODEC(T64, ZSTD),          -- FTS time ms (for INPUTBYSTRING)
    ftext_search_count UInt32 CODEC(T64, ZSTD),   -- FTS search count (for INPUTBYSTRING)
    ftext_result_count UInt32 CODEC(T64, ZSTD),   -- FTS result count (for INPUTBYSTRING)
    search_by_mask UInt8 CODEC(T64, ZSTD),        -- Search by mask (for INPUTBYSTRING)
    too_many_results UInt8 CODEC(T64, ZSTD),     -- Too many results (for INPUTBYSTRING)
    fill_refs_present UInt8 CODEC(T64, ZSTD),     -- Fill refs present (for INPUTBYSTRING)
    fts_job_id String CODEC(ZSTD),                -- FTS job ID
    fts_log_from String CODEC(ZSTD),              -- FTS log from timestamp
    fts_log_to String CODEC(ZSTD),                -- FTS log to timestamp
    fts_fixed_state String CODEC(ZSTD),          -- FTS fixed state
    fts_record_count UInt64 CODEC(T64, ZSTD),     -- FTS record count
    fts_total_records UInt64 CODEC(T64, ZSTD),    -- FTS total records
    fts_table_count UInt32 CODEC(T64, ZSTD),      -- FTS table count
    fts_table_name String CODEC(ZSTD),            -- FTS table name
    fts_table_code String CODEC(ZSTD),            -- FTS table code
    fts_table_ref String CODEC(ZSTD),             -- FTS table ref
    fts_metadata_id String CODEC(ZSTD),           -- FTS metadata ID
    fts_record_ref String CODEC(ZSTD),            -- FTS record ref
    fts_full_key String CODEC(ZSTD),              -- FTS full key
    fts_reindex_count UInt32 CODEC(T64, ZSTD),    -- FTS reindex count
    fts_skipped_records UInt64 CODEC(T64, ZSTD),  -- FTS skipped records
    fts_parallelism UInt32 CODEC(T64, ZSTD),      -- FTS parallelism
    
    -- Storage properties (STORE)
    store_id String CODEC(ZSTD),                 -- Storage object ID
    store_size UInt64 CODEC(T64, ZSTD),           -- Storage object size
    storage_guid String CODEC(ZSTD),             -- Storage GUID
    backup_file_name String CODEC(ZSTD),         -- Backup file name
    backup_base_file_name String CODEC(ZSTD),    -- Backup base file name
    backup_type UInt8 CODEC(T64, ZSTD),          -- Backup type (0-full, 1-diff)
    minimal_write_size UInt64 CODEC(T64, ZSTD),  -- Minimal write size
    read_only_mode UInt8 CODEC(T64, ZSTD),       -- Read-only mode
    use_mode String CODEC(ZSTD),                  -- Use mode
    
    -- Garbage collector properties (SDGC)
    sdgc_instance_id UInt64 CODEC(T64, ZSTD),     -- SDGC instance ID
    sdgc_method String CODEC(ZSTD),              -- SDGC method (Compact/Analyze)
    sdgc_files_size UInt64 CODEC(T64, ZSTD),     -- SDGC files size
    sdgc_used_size UInt64 CODEC(T64, ZSTD),      -- SDGC used size
    sdgc_copy_bytes UInt64 CODEC(T64, ZSTD),     -- SDGC copy bytes
    sdgc_lock_duration UInt64 CODEC(T64, ZSTD),  -- SDGC lock duration ms
    
    -- Add-in properties (ADDIN)
    addin_classes String CODEC(ZSTD),            -- Add-in classes
    addin_location String CODEC(ZSTD),            -- Add-in location
    addin_method_name String CODEC(ZSTD),         -- Add-in method name
    addin_message String CODEC(ZSTD),             -- Add-in message
    addin_source String CODEC(ZSTD),              -- Add-in source
    addin_type String CODEC(ZSTD),                -- Add-in type
    addin_result UInt8 CODEC(T64, ZSTD),         -- Add-in result (0-success, 1-error)
    addin_crashed UInt8 CODEC(T64, ZSTD),        -- Add-in crashed flag
    addin_error_descr String CODEC(ZSTD),       -- Add-in error description
    
    -- System event properties (SYSTEM)
    system_class String CODEC(ZSTD),              -- System class name
    system_component String CODEC(ZSTD),         -- System component name
    system_file String CODEC(ZSTD),               -- System file name
    system_line UInt32 CODEC(T64, ZSTD),          -- System line number
    system_txt String CODEC(ZSTD),                -- System text message
    
    -- Event log properties (EVENTLOG)
    eventlog_file_name String CODEC(ZSTD),       -- Event log file name
    eventlog_cpu_time UInt64 CODEC(T64, ZSTD),   -- Event log CPU time
    eventlog_os_thread UInt32 CODEC(T64, ZSTD),  -- Event log OS thread
    eventlog_packet_count UInt32 CODEC(T64, ZSTD), -- Event log packet count
    
    -- Video properties (VIDEOCALL, VIDEOCONN, VIDEOSTATS)
    video_connection String CODEC(ZSTD),         -- Video connection
    video_status String CODEC(ZSTD),              -- Video status
    video_stream_type String CODEC(ZSTD),         -- Video stream type
    video_value String CODEC(ZSTD),               -- Video value/statistics
    video_cpu UInt32 CODEC(T64, ZSTD),            -- Video CPU load
    video_queue_length UInt32 CODEC(T64, ZSTD),   -- Video queue length
    video_in_message String CODEC(ZSTD),          -- Video in message
    video_out_message String CODEC(ZSTD),        -- Video out message
    video_direction String CODEC(ZSTD),          -- Video direction
    video_type String CODEC(ZSTD),               -- Video type
    
    -- Speech recognition properties (STT, STTAdm)
    stt_id String CODEC(ZSTD),                   -- STT session ID
    stt_key String CODEC(ZSTD),                  -- STT session key
    stt_model_id String CODEC(ZSTD),              -- STT model ID
    stt_path String CODEC(ZSTD),                 -- STT component path
    stt_audio_encoding String CODEC(ZSTD),        -- STT audio encoding
    stt_frames UInt32 CODEC(T64, ZSTD),           -- STT frames count
    stt_contexts UInt32 CODEC(T64, ZSTD),         -- STT contexts count
    stt_contexts_only UInt8 CODEC(T64, ZSTD),    -- STT contexts only flag
    stt_recording UInt8 CODEC(T64, ZSTD),        -- STT recording flag
    stt_status String CODEC(ZSTD),               -- STT status
    stt_phrase String CODEC(ZSTD),               -- STT phrase
    stt_rx_acoustic String CODEC(ZSTD),          -- STT rx:Acoustic
    stt_rx_grammar String CODEC(ZSTD),            -- STT rx:Grammar
    stt_rx_language String CODEC(ZSTD),          -- STT rx:Language
    stt_rx_location String CODEC(ZSTD),           -- STT rx:Location
    stt_rx_sample_rate UInt32 CODEC(T64, ZSTD),  -- STT rx:SampleRate
    stt_rx_version String CODEC(ZSTD),            -- STT rx:Version
    stt_tx_acoustic String CODEC(ZSTD),           -- STT tx:Acoustic
    stt_tx_grammar String CODEC(ZSTD),           -- STT tx:Grammar
    stt_tx_language String CODEC(ZSTD),          -- STT tx:Language
    stt_tx_location String CODEC(ZSTD),           -- STT tx:Location
    stt_tx_sample_rate UInt32 CODEC(T64, ZSTD),  -- STT tx:SampleRate
    stt_tx_version String CODEC(ZSTD),           -- STT tx:Version
    
    -- Web service properties (VRSREQUEST, VRSRESPONSE)
    vrs_uri String CODEC(ZSTD),                  -- VRS URI
    vrs_method String CODEC(ZSTD),               -- VRS HTTP method
    vrs_headers String CODEC(ZSTD),              -- VRS headers
    vrs_body UInt64 CODEC(T64, ZSTD),            -- VRS body size
    vrs_status UInt32 CODEC(T64, ZSTD),          -- VRS HTTP status
    vrs_phrase String CODEC(ZSTD),              -- VRS status phrase
    
    -- Integration properties (SINTEG, EDS)
    sinteg_srvc_name String CODEC(ZSTD),         -- SINTEG service name
    sinteg_ext_srvc_url String CODEC(ZSTD),      -- SINTEG external service URL
    sinteg_ext_srvc_usr String CODEC(ZSTD),      -- SINTEG external service user
    
    -- Mail properties (MAILPARSEERR)
    mail_message_uid String CODEC(ZSTD),         -- Mail message UID
    mail_method String CODEC(ZSTD),              -- Mail method (GET/GETHEADERS/SETRAW)
    
    -- Certificate properties (WINCERT)
    win_cert_certificate String CODEC(ZSTD),      -- Certificate description
    win_cert_error_code UInt32 CODEC(T64, ZSTD), -- Certificate error code
    
    -- Data history properties (DHIST)
    dhist_description String CODEC(ZSTD),        -- DHIST description
    
    -- Config load properties (CONFLOADFROMFILES)
    conf_load_action String CODEC(ZSTD),         -- Config load action
    
    -- Background job properties
    report String CODEC(ZSTD),                   -- Report metadata name
    
    -- Client properties (t: prefix)
    t_application_name String CODEC(ZSTD),       -- t:applicationName
    t_client_id UInt64 CODEC(T64, ZSTD),         -- t:clientID
    t_computer_name String CODEC(ZSTD),          -- t:computerName
    t_connect_id UInt64 CODEC(T64, ZSTD),        -- t:connectID
    
    -- Additional properties
    host String CODEC(ZSTD),                     -- Host name
    val String CODEC(ZSTD),                       -- Value (depends on Func)
    err UInt8 CODEC(T64, ZSTD),                  -- Error type (0-info, 1-error)
    calls UInt32 CODEC(T64, ZSTD),                -- Calls count
    in_bytes UInt64 CODEC(T64, ZSTD),            -- Input bytes
    out_bytes UInt64 CODEC(T64, ZSTD),           -- Output bytes
    duration_us UInt64 CODEC(T64, ZSTD),         -- Duration in microseconds (Durationus)

    -- Dynamic properties (all other fields from tech log)
    property_key Array(String) CODEC(ZSTD),
    property_value Array(String) CODEC(ZSTD),
    
    -- Хеш записи для дедупликации
    record_hash String CODEC(ZSTD)  -- SHA1 hash (40 hex characters)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(ts)
ORDER BY (cluster_guid, infobase_guid, name, ts, record_hash)
TTL ts + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Indexes for common queries
ALTER TABLE logs.tech_log ADD INDEX idx_name name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_session session_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_duration duration TYPE minmax GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_hash record_hash TYPE bloom_filter(0.01) GRANULARITY 4;
-- Indexes for SQL queries (for performance analysis)
ALTER TABLE logs.tech_log ADD INDEX idx_dbms dbms TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX idx_exception exception TYPE set(0) GRANULARITY 4;

