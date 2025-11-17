package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/domain"
	"github.com/1c-log-checker/internal/logreader"
	"github.com/1c-log-checker/internal/logreader/eventlog"
	"github.com/1c-log-checker/internal/offset"
	"github.com/1c-log-checker/internal/techlog"
	"github.com/1c-log-checker/internal/writer"
	"github.com/rs/zerolog/log"
)

// ParserService orchestrates log parsing workers
type ParserService struct {
	cfg         *config.Config
	offsetStore offset.OffsetStore
	writer      writer.BatchWriter
	debugFile   *os.File // File for saving all parsed records
	debugMutex  sync.Mutex
	debugCount  int64    // Counter for records written to debug file

	wg sync.WaitGroup
}

// NewParserService creates a new parser service
func NewParserService(cfg *config.Config) (*ParserService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Initialize offset storage
	offsetStore, err := offset.NewBoltDBStore("offsets/parser.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create offset store: %w", err)
	}

	// Connect to ClickHouse (if not in READ_ONLY mode)
	var batchWriter writer.BatchWriter
	if !cfg.ReadOnly {
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{fmt.Sprintf("%s:%d", cfg.ClickHouseHost, cfg.ClickHousePort)},
			Auth: clickhouse.Auth{
				Database: cfg.ClickHouseDB,
				Username: "default",
				Password: "",
			},
		})
		if err != nil {
			offsetStore.Close()
			return nil, fmt.Errorf("failed to connect to clickhouse: %w", err)
		}

		batchWriter = writer.NewClickHouseWriter(conn, writer.BatchConfig{
			MaxSize:            cfg.BatchSize,
			FlushTimeout:       int64(cfg.BatchFlushTimeout),
			EnableDeduplication: cfg.EnableDeduplication,
		})
	}

	// Open debug file for saving all parsed records (only if enabled)
	var debugFile *os.File
	debugEnabled := os.Getenv("DEBUG_SAVE_ALL_RECORDS")
	if debugEnabled == "true" || debugEnabled == "1" {
		debugFile, err = os.OpenFile("/app/debug/parser_all_records.jsonl", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to open debug file, records won't be saved")
			debugFile = nil
		} else {
			log.Info().
				Str("file", "parser_all_records.jsonl").
				Msg("Debug mode enabled: All parsed records will be saved")
		}
	} else {
		log.Debug().Msg("Debug save disabled (DEBUG_SAVE_ALL_RECORDS not set)")
	}

	return &ParserService{
		cfg:         cfg,
		offsetStore: offsetStore,
		writer:      batchWriter,
		debugFile:   debugFile,
	}, nil
}

// Start starts the parser service
func (s *ParserService) Start(ctx context.Context) error {
	log.Info().
		Bool("read_only", s.cfg.ReadOnly).
		Int("event_log_dirs", len(s.cfg.LogDirs)).
		Int("tech_log_dirs", len(s.cfg.TechLogDirs)).
		Msg("Parser service starting...")
	
	// Scan for event logs
	if len(s.cfg.LogDirs) > 0 {
		locations, err := logreader.ScanForLogs(s.cfg.LogDirs)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan for event logs")
		} else {
			log.Info().Int("locations", len(locations)).Msg("Found event log locations")
			
			// Start reader for each location
			for _, loc := range locations {
				s.wg.Add(1)
				go func(location logreader.LogLocation) {
					defer s.wg.Done()
					s.runEventLogReader(ctx, location)
				}(loc)
			}
		}
	}
	
	// Start tech log tailers
	for _, dir := range s.cfg.TechLogDirs {
		s.wg.Add(1)
		go func(directory string) {
			defer s.wg.Done()
			s.runTechLogTailer(ctx, directory)
		}(dir)
	}
	
	log.Info().Msg("Parser service workers started")
	
	// Wait for context cancellation
	<-ctx.Done()
	
	log.Info().Msg("Parser service context cancelled, waiting for workers...")
	s.wg.Wait()
	
	return ctx.Err()
}

// Stop stops the parser service gracefully
// This MUST be called to release BoltDB file lock
func (s *ParserService) Stop() error {
	log.Info().Msg("Parser service stopping...")
	
	// Flush pending batches first
	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			log.Error().Err(err).Msg("Error flushing writer")
		}
	}
	
	// Close offset store - CRITICAL: releases BoltDB file lock
	if s.offsetStore != nil {
		if err := s.offsetStore.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing offset store")
			return err // Return error so caller knows cleanup failed
		}
		log.Debug().Msg("Offset store closed, BoltDB file unlocked")
	}
	
	// Close debug file
	if s.debugFile != nil {
		if err := s.debugFile.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close debug file")
		} else {
			log.Info().
				Str("file", "parser_all_records.jsonl").
				Int64("total_records", s.debugCount).
				Msg("All parsed records saved to debug file")
		}
	}
	
	log.Info().Msg("Parser service stopped")
	return nil
}

// runEventLogReader runs an event log reader for a location
func (s *ParserService) runEventLogReader(ctx context.Context, location logreader.LogLocation) {
	// Start timing for entire parsing process (from file read to ClickHouse write)
	parsingStartTime := time.Now()
	
	log.Info().
		Str("cluster_guid", location.ClusterGUID).
		Str("cluster_name", location.ClusterName).
		Str("infobase_guid", location.InfobaseGUID).
		Str("infobase_name", location.InfobaseName).
		Str("path", location.BasePath).
		Int("lgp_files", len(location.LgpFiles)).
		Msg("Starting event log reader")

	// Create direct file parsing reader
	// Note: InfobaseName will be empty (not available from 1C files directly)
	reader, err := s.createDirectReader(location, location.ClusterName, location.InfobaseName)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create direct reader")
		return
	}
	
	defer reader.Close()
	
	// Open reader
	if err := reader.Open(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to open event log reader")
		return
	}
	
	// Read and process events
	batch := make([]*domain.EventLogRecord, 0, 500)
	
	for {
		select {
		case <-ctx.Done():
			// Flush pending records
			if !s.cfg.ReadOnly && s.writer != nil {
				if err := s.writer.Flush(ctx); err != nil {
					log.Error().Err(err).Msg("Failed to flush writer")
				}
			}
			return
		default:
			// Read next record
			record, err := reader.Read(ctx)
			if err != nil {
			if err == io.EOF || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "end of stream") {
				// Flush pending records before waiting for new data
				if !s.cfg.ReadOnly && s.writer != nil {
					if err := s.writer.Flush(ctx); err != nil {
						log.Error().Err(err).Msg("Failed to flush writer at EOF")
					} else {
						log.Debug().Msg("Flushed pending records at EOF")
					}
				}
				
				// End of stream - in READ_ONLY mode, we're done
				if s.cfg.ReadOnly {
					// Flush any remaining records before calculating final stats
					if s.writer != nil {
						if err := s.writer.Flush(ctx); err != nil {
							log.Error().Err(err).Msg("Failed to flush writer at end of parsing")
						}
					}
					
					// Calculate total parsing time (from file read to ClickHouse write completion)
					totalParsingTime := time.Since(parsingStartTime)
					totalRecords := s.debugCount
					recordsPerSec := float64(totalRecords) / totalParsingTime.Seconds()
					
					log.Info().
						Str("cluster_name", location.ClusterName).
						Str("infobase_name", location.InfobaseName).
						Int("lgp_files", len(location.LgpFiles)).
						Dur("total_parsing_time", totalParsingTime).
						Int64("total_records_parsed", totalRecords).
						Float64("records_per_second", recordsPerSec).
						Msg("Parsing completed: All records processed and written to ClickHouse")

					// Write performance metrics to ClickHouse
					if !s.cfg.ReadOnly && s.writer != nil {
						metrics := &domain.ParserMetrics{
							Timestamp:        time.Now(),
							ParserType:       "event_log",
							ClusterGUID:      location.ClusterGUID,
							ClusterName:      location.ClusterName,
							InfobaseGUID:     location.InfobaseGUID,
							InfobaseName:     location.InfobaseName,
							FilesProcessed:   uint32(len(location.LgpFiles)),
							RecordsParsed:    uint64(totalRecords),
							ParsingTimeMs:    uint64(totalParsingTime.Milliseconds()),
							RecordsPerSecond: recordsPerSec,
							StartTime:        parsingStartTime,
							EndTime:          time.Now(),
							ErrorCount:       0,
						}
						if err := s.writer.WriteParserMetrics(ctx, metrics); err != nil {
							log.Error().Err(err).Msg("Failed to write parser metrics")
						}
					}
					return
				}
				// In live mode, wait a bit and continue
				log.Debug().Msg("End of stream, waiting for new data...")
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
				log.Error().Err(err).Msg("Failed to read event log record")
				time.Sleep(1 * time.Second)
				continue
			}
			
			// Save all parsed records to debug file (before deduplication)
			if s.debugFile != nil {
				s.debugMutex.Lock()
				encoder := json.NewEncoder(s.debugFile)
				if err := encoder.Encode(record); err != nil {
					log.Warn().Err(err).Msg("Failed to write record to debug file")
				} else {
					s.debugCount++
					if s.debugCount%50 == 0 {
						elapsed := time.Since(parsingStartTime)
						recordsPerSec := float64(s.debugCount) / elapsed.Seconds()
						log.Info().
							Int64("debug_records", s.debugCount).
							Dur("elapsed_time", elapsed).
							Float64("records_per_second", recordsPerSec).
							Msg("Saved records to debug file")
					}
				}
				s.debugMutex.Unlock()
			}
			
			// Write record immediately (writer handles batching internally)
			if !s.cfg.ReadOnly && s.writer != nil {
				if err := s.writer.WriteEventLog(ctx, record); err != nil {
					log.Error().Err(err).Msg("Failed to write event log record")
				} else {
					log.Debug().Msg("Wrote event log record")
				}
			} else {
				// In READ_ONLY mode, log each record in detail for comparison
				batch = append(batch, record)
				
				// Log each record with full details for comparison with 1C configurator
				log.Info().
					Str("event_time", record.EventTime.Format("02.01.2006 15:04:05")).
					Str("level", record.Level).
					Str("event", record.Event).
					Str("event_presentation", record.EventPresentation).
					Str("user_name", record.UserName).
					Str("computer", record.Computer).
					Str("application", record.ApplicationPresentation).
					Uint64("session_id", record.SessionID).
					Str("transaction_status", record.TransactionStatus).
					Str("comment", record.Comment).
					Str("metadata", record.MetadataPresentation).
					Str("data_presentation", record.DataPresentation).
					Str("cluster_guid", record.ClusterGUID).
					Str("infobase_guid", record.InfobaseGUID).
					Msg("Parsed event log record")
				
				// Also log summary every 100 records
				if len(batch) >= 100 {
					log.Info().
						Int("total_count", len(batch)).
						Str("cluster_guid", record.ClusterGUID).
						Str("infobase_guid", record.InfobaseGUID).
						Msg("Batch summary (READ_ONLY mode)")
					batch = batch[:0]
				}
			}
		}
	}
}

// createDirectReader creates a direct file parsing reader
func (s *ParserService) createDirectReader(location logreader.LogLocation, clusterName, infobaseName string) (logreader.EventLogReader, error) {
	return eventlog.NewReader(location.BasePath, location.ClusterGUID, location.InfobaseGUID, clusterName, infobaseName, s.cfg.MaxWorkers)
}

// runTechLogTailer runs a tech log tailer for a directory
func (s *ParserService) runTechLogTailer(ctx context.Context, dir string) {
	log.Info().Str("dir", dir).Msg("Starting tech log tailer")
	
	// Extract cluster_guid and infobase_guid from directory path
	clusterGUID, infobaseGUID, err := techlog.ExtractGUIDsFromPath(dir)
	if err != nil {
		log.Warn().
			Err(err).
			Str("dir", dir).
			Msg("Failed to extract GUIDs from path, metrics will have empty GUIDs")
		clusterGUID = ""
		infobaseGUID = ""
	}
	
	// Auto-detect format: try logcfg.xml first, then fallback to first log file
	format, err := techlog.DetectFormatFromDirectory(dir, s.cfg.TechLogConfigDir)
	if err != nil {
		log.Warn().
			Err(err).
			Str("dir", dir).
			Msg("Failed to detect format, defaulting to text")
		format = "text"
	}
	
	isJSON := format == "json"
	log.Info().
		Str("dir", dir).
		Str("format", format).
		Bool("isJSON", isJSON).
		Msg("Detected tech log format")
	
	// Metrics tracking
	startTime := time.Now()
	var recordsParsed uint64 = 0
	var filesProcessed uint32 = 0
	
	// Track files processed (will be updated by tailer)
	// Note: We can't easily track files from handler, so we'll use a simple approach:
	// Count files at start and assume all are processed
	allFiles, err := filepath.Glob(filepath.Join(dir, "*.log"))
	if err == nil {
		filesProcessed = uint32(len(allFiles))
	}
	
	tailer := techlog.NewTailer(dir, isJSON, s.offsetStore, s.cfg.MaxWorkers)

	// Set callback to flush pending batches after historical files processing completes
	tailer.SetHistoricalCompleteCallback(func() {
		if s.writer != nil {
			if err := s.writer.Flush(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to flush tech log batches after historical processing")
			} else {
				log.Info().Msg("Flushed pending tech log batches after historical processing")
			}
		}
	})

	handler := func(record *domain.TechLogRecord) error {
		// Note: cluster_guid and infobase_guid are already extracted from path in tailer.go
		// and added to record before calling this handler

		// Increment records counter
		recordsParsed++

		// Write to ClickHouse
		if s.writer != nil {
			if err := s.writer.WriteTechLog(ctx, record); err != nil {
				log.Error().Err(err).Msg("Failed to write tech log record")
				return err
			}
		}

		return nil
	}
	
	// Start tailer in goroutine to allow metrics collection
	done := make(chan error, 1)
	go func() {
		done <- tailer.Start(ctx, handler)
	}()
	
	// Periodically write metrics (every 5 minutes)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	// Write metrics on exit
	defer func() {
		endTime := time.Now()
		totalTime := endTime.Sub(startTime)
		recordsPerSec := float64(recordsParsed) / totalTime.Seconds()
		
		metrics := &domain.ParserMetrics{
			Timestamp:        time.Now(),
			ParserType:       "tech_log",
			ClusterGUID:      clusterGUID,
			ClusterName:      "", // Not available from path
			InfobaseGUID:     infobaseGUID,
			InfobaseName:     "", // Not available from path
			FilesProcessed:   filesProcessed,
			RecordsParsed:    recordsParsed,
			ParsingTimeMs:    uint64(totalTime.Milliseconds()),
			RecordsPerSecond: recordsPerSec,
			StartTime:        startTime,
			EndTime:          endTime,
			ErrorCount:       0,
		}
		
		if !s.cfg.ReadOnly && s.writer != nil {
			if err := s.writer.WriteParserMetrics(ctx, metrics); err != nil {
				log.Error().Err(err).Msg("Failed to write parser metrics")
			}
		}
		
		log.Info().
			Uint32("files", filesProcessed).
			Uint64("records", recordsParsed).
			Dur("time", totalTime).
			Float64("records_per_sec", recordsPerSec).
			Msg("Tech log parsing metrics")
	}()
	
	// Wait for tailer or ticker
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-done:
			if err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Tech log tailer error")
			}
			return
		case <-ticker.C:
			// Write periodic metrics
			currentTime := time.Now()
			elapsed := currentTime.Sub(startTime)
			recordsPerSec := float64(recordsParsed) / elapsed.Seconds()
			
			metrics := &domain.ParserMetrics{
				Timestamp:        time.Now(),
				ParserType:       "tech_log",
				ClusterGUID:      clusterGUID,
				ClusterName:      "", // Not available from path
				InfobaseGUID:     infobaseGUID,
				InfobaseName:     "", // Not available from path
				FilesProcessed:   filesProcessed,
				RecordsParsed:    recordsParsed,
				ParsingTimeMs:    uint64(elapsed.Milliseconds()),
				RecordsPerSecond: recordsPerSec,
				StartTime:        startTime,
				EndTime:          currentTime,
				ErrorCount:       0,
			}
			
			if !s.cfg.ReadOnly && s.writer != nil {
				if err := s.writer.WriteParserMetrics(ctx, metrics); err != nil {
					log.Error().Err(err).Msg("Failed to write periodic parser metrics")
				}
			}
		}
	}
}

