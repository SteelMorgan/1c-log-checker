package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/1c-log-checker/internal/config"
	"github.com/1c-log-checker/internal/domain"
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
	
	// Start tech log tailers
	for _, dir := range s.cfg.TechLogDirs {
		s.wg.Add(1)
		go func(directory string) {
			defer s.wg.Done()
			s.runTechLogTailer(ctx, directory)
		}(dir)
	}
	
	// TODO: Start event log readers
	// for _, dir := range s.cfg.LogDirs {
	//     s.wg.Add(1)
	//     go func(directory string) {
	//         defer s.wg.Done()
	//         s.runEventLogReader(ctx, directory)
	//     }(dir)
	// }
	
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

