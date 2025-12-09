-- Полный init-скрипт для ClickHouse: создаёт БД, все таблицы, индексы и вью
-- Выполняется на чистом volume при первом старте контейнера clickhouse

-- 1) База
CREATE DATABASE IF NOT EXISTS logs;

-- 2) Журнал регистрации (event_log) — финальная схема с UTC, hash и normalized
CREATE TABLE IF NOT EXISTS logs.event_log (
    event_time DateTime64(6, 'UTC') CODEC(Delta, ZSTD),
    event_date Date MATERIALIZED toDate(event_time),

    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),

    level LowCardinality(String) CODEC(ZSTD),
    event String CODEC(ZSTD),
    event_presentation String CODEC(ZSTD),

    user_name String CODEC(ZSTD),
    user_id UUID CODEC(ZSTD),
    computer String CODEC(ZSTD),

    application LowCardinality(String) CODEC(ZSTD),
    application_presentation String CODEC(ZSTD),

    session_id UInt64 CODEC(T64, ZSTD),
    connection_id UInt64 CODEC(T64, ZSTD),
    connection String CODEC(ZSTD),

    transaction_status String CODEC(ZSTD),
    transaction_id String CODEC(ZSTD),
    transaction_number Int64 CODEC(T64, ZSTD),
    transaction_datetime DateTime64(6, 'UTC') CODEC(Delta, ZSTD),

    data_separation String CODEC(ZSTD),

    metadata_name String CODEC(ZSTD),
    metadata_presentation String CODEC(ZSTD),

    comment String CODEC(ZSTD),
    data String CODEC(ZSTD),
    data_presentation String CODEC(ZSTD),

    server String CODEC(ZSTD),
    primary_port UInt16 CODEC(T64, ZSTD),
    secondary_port UInt16 CODEC(T64, ZSTD),

    props_key Array(String) CODEC(ZSTD),
    props_value Array(String) CODEC(ZSTD),

    record_hash String CODEC(ZSTD),
    comment_normalized String CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(event_time)
ORDER BY (cluster_guid, infobase_guid, event_time, session_id, record_hash)
TTL event_time + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Индексы event_log
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_event event TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_event_presentation event_presentation TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_cluster_guid cluster_guid TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_cluster_name cluster_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_infobase_guid infobase_guid TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_infobase_name infobase_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_user_name user_name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_user_id user_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_computer computer TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_application application TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_application_presentation application_presentation TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_session session_id TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_connection_id connection_id TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_connection connection TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_transaction_status transaction_status TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_transaction_number transaction_number TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_transaction_datetime transaction_datetime TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_data_separation data_separation TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_metadata_name metadata_name TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_metadata_presentation metadata_presentation TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_comment comment TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_data data TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_data_presentation data_presentation TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_server server TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_primary_port primary_port TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_secondary_port secondary_port TYPE minmax GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_hash record_hash TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.event_log ADD INDEX IF NOT EXISTS idx_comment_normalized comment_normalized TYPE bloom_filter(0.01) GRANULARITY 4;

-- 3) Технологический журнал (tech_log) — финальная схема с raw_line_normalized
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

    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    raw_line String CODEC(ZSTD),
    raw_line_normalized String CODEC(ZSTD),

    sql String CODEC(ZSTD),
    plan_sql_text String CODEC(ZSTD),
    rows UInt64 CODEC(T64, ZSTD),
    rows_affected UInt64 CODEC(T64, ZSTD),
    dbms LowCardinality(String) CODEC(ZSTD),
    database String CODEC(ZSTD),
    dbpid String CODEC(ZSTD),
    db_copy String CODEC(ZSTD),
    n_params UInt32 CODEC(T64, ZSTD),
    mdx String CODEC(ZSTD),
    db_conn_id String CODEC(ZSTD),
    db_conn_str String CODEC(ZSTD),
    db_usr String CODEC(ZSTD),

    query String CODEC(ZSTD),
    sdbl String CODEC(ZSTD),
    query_fields String CODEC(ZSTD),

    exception String CODEC(ZSTD),
    exception_descr String CODEC(ZSTD),
    exception_context String CODEC(ZSTD),
    func String CODEC(ZSTD),
    line UInt32 CODEC(T64, ZSTD),
    file String CODEC(ZSTD),
    module String CODEC(ZSTD),
    os_exception String CODEC(ZSTD),

    locks String CODEC(ZSTD),
    regions String CODEC(ZSTD),
    wait_connections String CODEC(ZSTD),
    lka String CODEC(ZSTD),
    lkp String CODEC(ZSTD),
    lkpid String CODEC(ZSTD),
    lkaid String CODEC(ZSTD),
    lksrc String CODEC(ZSTD),
    lkpto UInt64 CODEC(T64, ZSTD),
    lkato UInt64 CODEC(T64, ZSTD),
    deadlock_connection_intersections String CODEC(ZSTD),

    server String CODEC(ZSTD),
    port UInt32 CODEC(T64, ZSTD),
    sync_port UInt32 CODEC(T64, ZSTD),
    connection UInt64 CODEC(T64, ZSTD),
    h_result_oledb String CODEC(ZSTD),
    h_result_nc2005 String CODEC(ZSTD),
    h_result_nc2008 String CODEC(ZSTD),
    h_result_nc2012 String CODEC(ZSTD),

    session_nmb UInt64 CODEC(T64, ZSTD),
    seance_id String CODEC(ZSTD),

    proc_id String CODEC(ZSTD),
    pid UInt32 CODEC(T64, ZSTD),
    process_name String CODEC(ZSTD),
    p_process_name String CODEC(ZSTD),
    src_process_name String CODEC(ZSTD),
    finish String CODEC(ZSTD),
    exit_code Int32 CODEC(T64, ZSTD),
    run_as String CODEC(ZSTD),

    m_name String CODEC(ZSTD),
    i_name String CODEC(ZSTD),
    dst_client_id UInt64 CODEC(T64, ZSTD),
    ret_excp String CODEC(ZSTD),
    memory UInt64 CODEC(T64, ZSTD),
    memory_peak UInt64 CODEC(T64, ZSTD),

    cluster_event String CODEC(ZSTD),
    cluster UInt32 CODEC(T64, ZSTD),
    ib String CODEC(ZSTD),
    ref String CODEC(ZSTD),
    connections UInt32 CODEC(T64, ZSTD),
    conn_limit UInt32 CODEC(T64, ZSTD),
    infobases UInt32 CODEC(T64, ZSTD),
    ib_limit UInt32 CODEC(T64, ZSTD),
    dst_addr String CODEC(ZSTD),
    dst_id String CODEC(ZSTD),
    dst_pid UInt32 CODEC(T64, ZSTD),
    dst_srv String CODEC(ZSTD),
    src_addr String CODEC(ZSTD),
    src_id String CODEC(ZSTD),
    src_pid UInt32 CODEC(T64, ZSTD),
    src_srv String CODEC(ZSTD),
    src_url String CODEC(ZSTD),
    my_ver String CODEC(ZSTD),
    src_ver String CODEC(ZSTD),
    registered String CODEC(ZSTD),
    obsolete String CODEC(ZSTD),
    released String CODEC(ZSTD),
    reason String CODEC(ZSTD),
    request String CODEC(ZSTD),
    service_name String CODEC(ZSTD),
    application_ext String CODEC(ZSTD),
    need_resync String CODEC(ZSTD),
    new_service_data_directory String CODEC(ZSTD),
    old_service_data_directory String CODEC(ZSTD),

    server_computer_name String CODEC(ZSTD),
    proc_url String CODEC(ZSTD),
    agent_url String CODEC(ZSTD),

    admin String CODEC(ZSTD),
    action String CODEC(ZSTD),

    sz UInt64 CODEC(T64, ZSTD),
    szd Int64 CODEC(T64, ZSTD),
    cn UInt32 CODEC(T64, ZSTD),
    cnd Int32 CODEC(T64, ZSTD),
    memory_limits String CODEC(ZSTD),
    excess_duration_sec UInt64 CODEC(T64, ZSTD),
    excess_start_time DateTime64(6) CODEC(Delta, ZSTD),
    free_memory UInt64 CODEC(T64, ZSTD),
    total_memory UInt64 CODEC(T64, ZSTD),
    safe_limit UInt64 CODEC(T64, ZSTD),
    attn_info String CODEC(ZSTD),
    attn_pid UInt32 CODEC(T64, ZSTD),
    attn_process_id String CODEC(ZSTD),
    attn_server_id String CODEC(ZSTD),
    attn_url String CODEC(ZSTD),

    lic_res String CODEC(ZSTD),
    hasp_id String CODEC(ZSTD),

    ftext_state String CODEC(ZSTD),
    av_mem UInt64 CODEC(T64, ZSTD),
    background_job_created UInt8 CODEC(T64, ZSTD),
    memory_used UInt64 CODEC(T64, ZSTD),
    failed_jobs_count UInt32 CODEC(T64, ZSTD),
    total_jobs_count UInt32 CODEC(T64, ZSTD),
    job_canceled_by_load_limit UInt8 CODEC(T64, ZSTD),
    min_data_id UInt64 CODEC(T64, ZSTD),
    ftext_files String CODEC(ZSTD),
    ftext_files_count UInt32 CODEC(T64, ZSTD),
    ftext_files_total_size UInt64 CODEC(T64, ZSTD),
    ftext_folder String CODEC(ZSTD),
    ftext_time String CODEC(ZSTD),
    ftext_file String CODEC(ZSTD),
    ftext_info String CODEC(ZSTD),
    ftext_result UInt8 CODEC(T64, ZSTD),
    ftext_separation UInt8 CODEC(T64, ZSTD),
    ftext_sep_id UInt32 CODEC(T64, ZSTD),
    ftext_word String CODEC(ZSTD),
    find_by_string String CODEC(ZSTD),
    input_text String CODEC(ZSTD),
    find_ticks UInt64 CODEC(T64, ZSTD),
    ftext_ticks UInt64 CODEC(T64, ZSTD),
    ftext_search_count UInt32 CODEC(T64, ZSTD),
    ftext_result_count UInt32 CODEC(T64, ZSTD),
    search_by_mask UInt8 CODEC(T64, ZSTD),
    too_many_results UInt8 CODEC(T64, ZSTD),
    fill_refs_present UInt8 CODEC(T64, ZSTD),
    fts_job_id String CODEC(ZSTD),
    fts_log_from String CODEC(ZSTD),
    fts_log_to String CODEC(ZSTD),
    fts_fixed_state String CODEC(ZSTD),
    fts_record_count UInt64 CODEC(T64, ZSTD),
    fts_total_records UInt64 CODEC(T64, ZSTD),
    fts_table_count UInt32 CODEC(T64, ZSTD),
    fts_table_name String CODEC(ZSTD),
    fts_table_code String CODEC(ZSTD),
    fts_table_ref String CODEC(ZSTD),
    fts_metadata_id String CODEC(ZSTD),
    fts_record_ref String CODEC(ZSTD),
    fts_full_key String CODEC(ZSTD),
    fts_reindex_count UInt32 CODEC(T64, ZSTD),
    fts_skipped_records UInt64 CODEC(T64, ZSTD),
    fts_parallelism UInt32 CODEC(T64, ZSTD),

    store_id String CODEC(ZSTD),
    store_size UInt64 CODEC(T64, ZSTD),
    storage_guid String CODEC(ZSTD),
    backup_file_name String CODEC(ZSTD),
    backup_base_file_name String CODEC(ZSTD),
    backup_type UInt8 CODEC(T64, ZSTD),
    minimal_write_size UInt64 CODEC(T64, ZSTD),
    read_only_mode UInt8 CODEC(T64, ZSTD),
    use_mode String CODEC(ZSTD),

    sdgc_instance_id UInt64 CODEC(T64, ZSTD),
    sdgc_method String CODEC(ZSTD),
    sdgc_files_size UInt64 CODEC(T64, ZSTD),
    sdgc_used_size UInt64 CODEC(T64, ZSTD),
    sdgc_copy_bytes UInt64 CODEC(T64, ZSTD),
    sdgc_lock_duration UInt64 CODEC(T64, ZSTD),

    addin_classes String CODEC(ZSTD),
    addin_location String CODEC(ZSTD),
    addin_method_name String CODEC(ZSTD),
    addin_message String CODEC(ZSTD),
    addin_source String CODEC(ZSTD),
    addin_type String CODEC(ZSTD),
    addin_result UInt8 CODEC(T64, ZSTD),
    addin_crashed UInt8 CODEC(T64, ZSTD),
    addin_error_descr String CODEC(ZSTD),

    system_class String CODEC(ZSTD),
    system_component String CODEC(ZSTD),
    system_file String CODEC(ZSTD),
    system_line UInt32 CODEC(T64, ZSTD),
    system_txt String CODEC(ZSTD),

    eventlog_file_name String CODEC(ZSTD),
    eventlog_cpu_time UInt64 CODEC(T64, ZSTD),
    eventlog_os_thread UInt32 CODEC(T64, ZSTD),
    eventlog_packet_count UInt32 CODEC(T64, ZSTD),

    video_connection String CODEC(ZSTD),
    video_status String CODEC(ZSTD),
    video_stream_type String CODEC(ZSTD),
    video_value String CODEC(ZSTD),
    video_cpu UInt32 CODEC(T64, ZSTD),
    video_queue_length UInt32 CODEC(T64, ZSTD),
    video_in_message String CODEC(ZSTD),
    video_out_message String CODEC(ZSTD),
    video_direction String CODEC(ZSTD),
    video_type String CODEC(ZSTD),

    stt_id String CODEC(ZSTD),
    stt_key String CODEC(ZSTD),
    stt_model_id String CODEC(ZSTD),
    stt_path String CODEC(ZSTD),
    stt_audio_encoding String CODEC(ZSTD),
    stt_frames UInt32 CODEC(T64, ZSTD),
    stt_contexts UInt32 CODEC(T64, ZSTD),
    stt_contexts_only UInt8 CODEC(T64, ZSTD),
    stt_recording UInt8 CODEC(T64, ZSTD),
    stt_status String CODEC(ZSTD),
    stt_phrase String CODEC(ZSTD),
    stt_rx_acoustic String CODEC(ZSTD),
    stt_rx_grammar String CODEC(ZSTD),
    stt_rx_language String CODEC(ZSTD),
    stt_rx_location String CODEC(ZSTD),
    stt_rx_sample_rate UInt32 CODEC(T64, ZSTD),
    stt_rx_version String CODEC(ZSTD),
    stt_tx_acoustic String CODEC(ZSTD),
    stt_tx_grammar String CODEC(ZSTD),
    stt_tx_language String CODEC(ZSTD),
    stt_tx_location String CODEC(ZSTD),
    stt_tx_sample_rate UInt32 CODEC(T64, ZSTD),
    stt_tx_version String CODEC(ZSTD),

    vrs_uri String CODEC(ZSTD),
    vrs_method String CODEC(ZSTD),
    vrs_headers String CODEC(ZSTD),
    vrs_body UInt64 CODEC(T64, ZSTD),
    vrs_status UInt32 CODEC(T64, ZSTD),
    vrs_phrase String CODEC(ZSTD),

    sinteg_srvc_name String CODEC(ZSTD),
    sinteg_ext_srvc_url String CODEC(ZSTD),
    sinteg_ext_srvc_usr String CODEC(ZSTD),

    mail_message_uid String CODEC(ZSTD),
    mail_method String CODEC(ZSTD),

    win_cert_certificate String CODEC(ZSTD),
    win_cert_error_code UInt32 CODEC(T64, ZSTD),

    dhist_description String CODEC(ZSTD),

    conf_load_action String CODEC(ZSTD),

    report String CODEC(ZSTD),

    t_application_name String CODEC(ZSTD),
    t_client_id UInt64 CODEC(T64, ZSTD),
    t_computer_name String CODEC(ZSTD),
    t_connect_id UInt64 CODEC(T64, ZSTD),

    host String CODEC(ZSTD),
    val String CODEC(ZSTD),
    err UInt8 CODEC(T64, ZSTD),
    calls UInt32 CODEC(T64, ZSTD),
    in_bytes UInt64 CODEC(T64, ZSTD),
    out_bytes UInt64 CODEC(T64, ZSTD),
    duration_us UInt64 CODEC(T64, ZSTD),

    property_key Array(String) CODEC(ZSTD),
    property_value Array(String) CODEC(ZSTD),

    record_hash String CODEC(ZSTD)
) ENGINE = MergeTree()
PARTITION BY toYYYYMMDD(ts)
ORDER BY (cluster_guid, infobase_guid, name, ts, record_hash)
TTL ts + INTERVAL 30 DAY
SETTINGS index_granularity = 8192;

-- Индексы tech_log
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_name name TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_level level TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_session session_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_transaction transaction_id TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_duration duration TYPE minmax GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_hash record_hash TYPE bloom_filter(0.01) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_dbms dbms TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_exception exception TYPE set(0) GRANULARITY 4;
ALTER TABLE logs.tech_log ADD INDEX IF NOT EXISTS idx_raw_line_normalized raw_line_normalized TYPE bloom_filter(0.01) GRANULARITY 4;

-- 4) Метрики парсера (parser_metrics) — финальная версия ReplacingMergeTree
CREATE TABLE IF NOT EXISTS logs.parser_metrics (
    timestamp DateTime DEFAULT now(),
    parser_type LowCardinality(String),
    cluster_guid String,
    cluster_name String,
    infobase_guid String,
    infobase_name String,
    file_path String DEFAULT '',
    file_name String DEFAULT '',
    files_processed UInt32,
    records_parsed UInt64,
    parsing_time_ms UInt64,
    records_per_second Float64,
    start_time DateTime,
    end_time DateTime,
    error_count UInt32 DEFAULT 0,
    file_reading_time_ms UInt64 DEFAULT 0,
    record_parsing_time_ms UInt64 DEFAULT 0,
    deduplication_time_ms UInt64 DEFAULT 0,
    writing_time_ms UInt64 DEFAULT 0,
    updated_at DateTime DEFAULT now()
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path)
TTL updated_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;

CREATE INDEX IF NOT EXISTS idx_parser_metrics_timestamp ON logs.parser_metrics (timestamp) TYPE minmax GRANULARITY 1;
CREATE INDEX IF NOT EXISTS idx_parser_metrics_type ON logs.parser_metrics (parser_type) TYPE set(0) GRANULARITY 1;
CREATE INDEX IF NOT EXISTS idx_parser_metrics_file_path ON logs.parser_metrics (file_path) TYPE bloom_filter(0.01) GRANULARITY 1;

-- 5) Вью с вычисляемыми метриками
CREATE OR REPLACE VIEW logs.parser_metrics_extended AS
SELECT
    parser_type,
    cluster_name,
    infobase_name,
    file_path,
    start_time,
    end_time,
    (parsing_time_ms + deduplication_time_ms + writing_time_ms) AS total_time_ms,
    records_parsed,
    records_per_second,
    file_reading_time_ms,
    record_parsing_time_ms,
    deduplication_time_ms,
    writing_time_ms,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((file_reading_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS file_reading_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((record_parsing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS parsing_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((deduplication_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS deduplication_percentage,
    CASE WHEN (parsing_time_ms + deduplication_time_ms + writing_time_ms) > 0
         THEN round((writing_time_ms * 100.0 / (parsing_time_ms + deduplication_time_ms + writing_time_ms)), 2) ELSE 0 END AS writing_percentage
FROM logs.parser_metrics
FINAL;

-- 6) Прогресс чтения файлов (file_reading_progress) — финальная версия
CREATE TABLE IF NOT EXISTS logs.file_reading_progress (
    timestamp DateTime64(6) DEFAULT now(),
    parser_type LowCardinality(String),
    cluster_guid String CODEC(ZSTD),
    cluster_name String CODEC(ZSTD),
    infobase_guid String CODEC(ZSTD),
    infobase_name String CODEC(ZSTD),
    file_path String CODEC(ZSTD),
    file_name String CODEC(ZSTD),
    file_size_bytes UInt64 CODEC(T64, ZSTD),
    offset_bytes UInt64 CODEC(T64, ZSTD),
    records_parsed UInt64 CODEC(T64, ZSTD),
    last_timestamp DateTime64(6) CODEC(Delta, ZSTD),
    progress_percent Float64 CODEC(ZSTD),
    updated_at DateTime64(6) DEFAULT now() CODEC(Delta, ZSTD)
) ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)
ORDER BY (parser_type, cluster_guid, infobase_guid, file_path)
TTL updated_at + INTERVAL 90 DAY
SETTINGS index_granularity = 8192;



