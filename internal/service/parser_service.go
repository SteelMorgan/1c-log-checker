package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/domain"
	"github.com/1c-log-checker/internal/logreader"
	"github.com/1c-log-checker/internal/logreader/eventlog"
	"github.com/1c-log-checker/internal/mapping"
	"github.com/1c-log-checker/internal/offset"
	"github.com/1c-log-checker/internal/techlog"
	"github.com/1c-log-checker/internal/writer"
	"github.com/rs/zerolog/log"
)

// ParserService orchestrates log parsing workers
type ParserService struct {
	cfg        *config.Config
	offsetStore offset.OffsetStore
	writer     writer.BatchWriter
	clusterMap *mapping.ClusterMap
	
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

	// Load cluster map
	clusterMap, err := mapping.LoadClusterMap(cfg.ClusterMapPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load cluster map, using GUIDs as names")
		clusterMap = &mapping.ClusterMap{
			Clusters:  make(map[string]mapping.ClusterInfo),
			Infobases: make(map[string]mapping.InfobaseInfo),
		}
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
			MaxSize:      500,
			FlushTimeout: 100, // 100ms
		})
	}

	return &ParserService{
		cfg:        cfg,
		offsetStore: offsetStore,
		writer:     batchWriter,
		clusterMap: clusterMap,
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
func (s *ParserService) Stop() error {
	log.Info().Msg("Parser service stopping...")
	
	// Flush pending batches
	if s.writer != nil {
		if err := s.writer.Close(); err != nil {
			log.Error().Err(err).Msg("Error flushing writer")
		}
	}
	
	// Close offset store
	if s.offsetStore != nil {
		if err := s.offsetStore.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing offset store")
		}
	}
	
	log.Info().Msg("Parser service stopped")
	return nil
}

// runEventLogReader runs an event log reader for a location
func (s *ParserService) runEventLogReader(ctx context.Context, location logreader.LogLocation) {
	log.Info().
		Str("cluster_guid", location.ClusterGUID).
		Str("infobase_guid", location.InfobaseGUID).
		Str("path", location.BasePath).
		Str("method", s.cfg.EventLogMethod).
		Int("lgp_files", len(location.LgpFiles)).
		Msg("Starting event log reader")
	
	// Try to use configured method, fallback to direct if ibcmd fails
	var reader logreader.EventLogReader
	var err error
	
	if s.cfg.EventLogMethod == "ibcmd" {
		reader, err = s.createIbcmdReader(location)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Failed to create ibcmd reader, falling back to direct parsing")
			// Fallback to direct parsing
			reader, err = s.createDirectReader(location)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create direct reader")
				return
			}
		}
	} else {
		// Direct parsing method
		reader, err = s.createDirectReader(location)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create direct reader")
			return
		}
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
				if err.Error() == "EOF" || err.Error() == "end of stream" {
					// End of stream, wait a bit and continue
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
			
			// Write record immediately (writer handles batching internally)
			if !s.cfg.ReadOnly && s.writer != nil {
				if err := s.writer.WriteEventLog(ctx, record); err != nil {
					log.Error().Err(err).Msg("Failed to write event log record")
				} else {
					log.Debug().Msg("Wrote event log record")
				}
			} else {
				// In READ_ONLY mode, just collect for logging
				batch = append(batch, record)
				if len(batch) >= 100 {
					log.Info().Int("count", len(batch)).Msg("Read event log records (READ_ONLY mode)")
					batch = batch[:0]
				}
			}
		}
	}
}

// createIbcmdReader creates an ibcmd-based reader
func (s *ParserService) createIbcmdReader(location logreader.LogLocation) (logreader.EventLogReader, error) {
	ibcmdPath := s.cfg.IbcmdPath
	
	// Auto-detect ibcmd if path not specified
	if ibcmdPath == "" {
		var err error
		ibcmdPath, err = eventlog.FindIbcmd()
		if err != nil {
			return nil, fmt.Errorf("ibcmd not found: %w", err)
		}
		log.Info().Str("ibcmd_path", ibcmdPath).Msg("Auto-detected ibcmd path")
	}
	
	// Verify ibcmd
	if err := eventlog.VerifyIbcmd(ibcmdPath); err != nil {
		return nil, fmt.Errorf("ibcmd verification failed: %w", err)
	}
	
	return eventlog.NewIbcmdReader(ibcmdPath, location.BasePath, location.ClusterGUID, location.InfobaseGUID)
}

// createDirectReader creates a direct file parsing reader
func (s *ParserService) createDirectReader(location logreader.LogLocation) (logreader.EventLogReader, error) {
	return eventlog.NewReader(location.BasePath, location.ClusterGUID, location.InfobaseGUID)
}

// runTechLogTailer runs a tech log tailer for a directory
func (s *ParserService) runTechLogTailer(ctx context.Context, dir string) {
	log.Info().Str("dir", dir).Msg("Starting tech log tailer")
	
	// Detect format (check for .json files or default to text)
	isJSON := false // TODO: Auto-detect from files
	
	tailer := techlog.NewTailer(dir, isJSON)
	
	handler := func(record *domain.TechLogRecord) error {
		// Enrich with cluster/infobase info (TODO: extract from path or config)
		// For now, leave empty
		
		// Write to ClickHouse
		if s.writer != nil {
			if err := s.writer.WriteTechLog(ctx, record); err != nil {
				log.Error().Err(err).Msg("Failed to write tech log record")
				return err
			}
		}
		
		return nil
	}
	
	if err := tailer.Start(ctx, handler); err != nil {
		log.Error().Err(err).Str("dir", dir).Msg("Tech log tailer error")
	}
}

