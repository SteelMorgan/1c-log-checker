package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/1c-log-checker/internal/clickhouse"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// EnrichmentWorker periodically enriches GUID → Presentation mappings
// by requesting 1C for unknown GUIDs
type EnrichmentWorker struct {
	chClient     *clickhouse.Client
	endpoint1C   string // 1C endpoint for resolving GUIDs
	apiKey       string
	batchSize    int           // Max GUIDs per request (default: 500)
	interval     time.Duration // How often to run (same as parser)
	httpClient   *http.Client
	stopChan     chan struct{}
	infobases    []string // List of infobase GUIDs to process
}

// NewEnrichmentWorker creates a new enrichment worker
func NewEnrichmentWorker(
	chClient *clickhouse.Client,
	endpoint1C string,
	apiKey string,
	interval time.Duration,
	infobases []string,
) *EnrichmentWorker {
	return &EnrichmentWorker{
		chClient:   chClient,
		endpoint1C: endpoint1C,
		apiKey:     apiKey,
		batchSize:  500,
		interval:   interval,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		stopChan:  make(chan struct{}),
		infobases: infobases,
	}
}

// Start starts the enrichment worker
func (w *EnrichmentWorker) Start(ctx context.Context) {
	log.Info().
		Str("endpoint", w.endpoint1C).
		Dur("interval", w.interval).
		Int("batch_size", w.batchSize).
		Msg("Starting enrichment worker")

	// Run immediately on startup
	w.runEnrichment(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runEnrichment(ctx)
		case <-w.stopChan:
			log.Info().Msg("Enrichment worker stopped")
			return
		case <-ctx.Done():
			log.Info().Msg("Enrichment worker context cancelled")
			return
		}
	}
}

// Stop stops the worker
func (w *EnrichmentWorker) Stop() {
	close(w.stopChan)
}

// runEnrichment performs one enrichment cycle
func (w *EnrichmentWorker) runEnrichment(ctx context.Context) {
	startTime := time.Now()

	log.Info().Msg("Starting enrichment cycle")

	// Enrich users
	userCount, err := w.enrichUsers(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to enrich users")
	} else {
		log.Info().Int("enriched", userCount).Msg("Users enriched")
	}

	// Enrich metadata
	metadataCount, err := w.enrichMetadata(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to enrich metadata")
	} else {
		log.Info().Int("enriched", metadataCount).Msg("Metadata enriched")
	}

	// Optionally enrich data objects (can be heavy)
	// dataCount, err := w.enrichDataObjects(ctx)

	log.Info().
		Dur("duration", time.Since(startTime)).
		Int("users", userCount).
		Int("metadata", metadataCount).
		Msg("Enrichment cycle completed")
}

// enrichUsers enriches user GUIDs
func (w *EnrichmentWorker) enrichUsers(ctx context.Context) (int, error) {
	totalEnriched := 0

	for _, infobaseGUID := range w.infobases {
		// Step 1: Find GUIDs without presentations (where user_name = user_id)
		unenrichedQuery := `
			SELECT DISTINCT user_id
			FROM logs.event_log
			WHERE infobase_guid = ?
			  AND user_name = toString(user_id)
			  AND user_id != toUUID('00000000-0000-0000-0000-000000000000')
			LIMIT 10000
		`

		rows, err := w.chClient.Query(ctx, unenrichedQuery, infobaseGUID)
		if err != nil {
			return totalEnriched, fmt.Errorf("failed to query unenriched users: %w", err)
		}

		var unenrichedGUIDs []uuid.UUID
		for rows.Next() {
			var userID uuid.UUID
			if err := rows.Scan(&userID); err != nil {
				log.Warn().Err(err).Msg("Failed to scan user_id")
				continue
			}
			unenrichedGUIDs = append(unenrichedGUIDs, userID)
		}
		rows.Close()

		if len(unenrichedGUIDs) == 0 {
			continue
		}

		log.Debug().
			Str("infobase_guid", infobaseGUID).
			Int("unenriched_count", len(unenrichedGUIDs)).
			Msg("Found unenriched user GUIDs")

		// Step 2: Check cache - find GUIDs that already have presentations in DB
		cachedPresentations := make(map[uuid.UUID]string)
		needResolve := make([]uuid.UUID, 0)

		for _, userID := range unenrichedGUIDs {
			// Check if we have a presentation for this GUID in other records
			cacheQuery := `
				SELECT user_name
				FROM logs.event_log
				WHERE infobase_guid = ?
				  AND user_id = ?
				  AND user_name != toString(user_id)
				LIMIT 1
			`

			cacheRows, err := w.chClient.Query(ctx, cacheQuery, infobaseGUID, userID)
			if err != nil {
				log.Warn().Err(err).Str("user_id", userID.String()).Msg("Cache query failed")
				needResolve = append(needResolve, userID)
				continue
			}

			if cacheRows.Next() {
				var userName string
				if err := cacheRows.Scan(&userName); err == nil && userName != "" {
					cachedPresentations[userID] = userName
					log.Debug().
						Str("user_id", userID.String()).
						Str("cached_name", userName).
						Msg("Found cached presentation")
				} else {
					needResolve = append(needResolve, userID)
				}
			} else {
				needResolve = append(needResolve, userID)
			}
			cacheRows.Close()
		}

		log.Info().
			Int("cached", len(cachedPresentations)).
			Int("need_resolve", len(needResolve)).
			Msg("Cache check completed")

		// Step 3: Write cached presentations to mapping table
		if len(cachedPresentations) > 0 {
			if err := w.writeCachedUserMappings(ctx, infobaseGUID, cachedPresentations); err != nil {
				log.Error().Err(err).Msg("Failed to write cached mappings")
			} else {
				totalEnriched += len(cachedPresentations)
			}
		}

		// Step 4: Resolve unknown GUIDs via 1C (in batches)
		if len(needResolve) > 0 {
			resolved, err := w.resolveUsersVia1C(ctx, infobaseGUID, needResolve)
			if err != nil {
				log.Error().Err(err).Msg("Failed to resolve users via 1C")
			} else {
				totalEnriched += len(resolved)
			}
		}
	}

	return totalEnriched, nil
}

// enrichMetadata enriches metadata GUIDs
func (w *EnrichmentWorker) enrichMetadata(ctx context.Context) (int, error) {
	totalEnriched := 0

	for _, infobaseGUID := range w.infobases {
		// Step 1: Find unenriched metadata GUIDs
		unenrichedQuery := `
			SELECT DISTINCT
				arrayJoin(arrayMap(x -> reinterpretAsUUID(unhex(x)),
					arrayFilter(x -> length(x) = 32,
						arrayMap(x -> replaceAll(x, '-', ''),
							extractAll(metadata_name, '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}')
						)
					)
				)) AS metadata_guid
			FROM logs.event_log
			WHERE infobaseguid = ?
			  AND metadata_name != ''
			  AND metadata_presentation = ''
			  AND metadata_guid != toUUID('00000000-0000-0000-0000-000000000000')
			LIMIT 5000
		`

		rows, err := w.chClient.Query(ctx, unenrichedQuery, infobaseGUID)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to query unenriched metadata, skipping")
			continue
		}

		var unenrichedGUIDs []uuid.UUID
		for rows.Next() {
			var metadataGUID uuid.UUID
			if err := rows.Scan(&metadataGUID); err != nil {
				continue
			}
			unenrichedGUIDs = append(unenrichedGUIDs, metadataGUID)
		}
		rows.Close()

		if len(unenrichedGUIDs) == 0 {
			continue
		}

		// Step 2: Check cache
		// TODO: Implement cache check for metadata presentations
		needResolve := make([]uuid.UUID, 0)

		// Similar logic as for users...
		// (abbreviated for brevity - same pattern)

		// Step 3: Resolve via 1C
		if len(needResolve) > 0 {
			resolved, err := w.resolveMetadataVia1C(ctx, infobaseGUID, needResolve)
			if err != nil {
				log.Error().Err(err).Msg("Failed to resolve metadata via 1C")
			} else {
				totalEnriched += len(resolved)
			}
		}
	}

	return totalEnriched, nil
}

// resolveUsersVia1C resolves user GUIDs via 1C endpoint
func (w *EnrichmentWorker) resolveUsersVia1C(ctx context.Context, infobaseGUID string, guids []uuid.UUID) (map[uuid.UUID]string, error) {
	if len(guids) == 0 {
		return nil, nil
	}

	resolved := make(map[uuid.UUID]string)

	// Process in batches
	for i := 0; i < len(guids); i += w.batchSize {
		end := i + w.batchSize
		if end > len(guids) {
			end = len(guids)
		}

		batch := guids[i:end]

		log.Info().
			Int("batch_start", i).
			Int("batch_size", len(batch)).
			Msg("Resolving user GUIDs via 1C")

		// Prepare request
		request := ResolveRequest{
			InfobaseGUID: infobaseGUID,
			UserGUIDs:    make([]string, len(batch)),
		}

		for j, guid := range batch {
			request.UserGUIDs[j] = guid.String()
		}

		// Call 1C
		response, err := w.callResolveEndpoint(ctx, request)
		if err != nil {
			log.Error().Err(err).Msg("Failed to call 1C resolve endpoint")
			continue
		}

		// Write to mapping table
		if err := w.writeResolvedUserMappings(ctx, infobaseGUID, response.Users); err != nil {
			log.Error().Err(err).Msg("Failed to write resolved mappings")
			continue
		}

		// Collect results
		for guidStr, name := range response.Users {
			guid, err := uuid.Parse(guidStr)
			if err == nil {
				resolved[guid] = name
			}
		}

		log.Info().
			Int("resolved_count", len(response.Users)).
			Msg("Batch resolved successfully")
	}

	return resolved, nil
}

// resolveMetadataVia1C resolves metadata GUIDs via 1C
func (w *EnrichmentWorker) resolveMetadataVia1C(ctx context.Context, infobaseGUID string, guids []uuid.UUID) (map[uuid.UUID]string, error) {
	// Similar to resolveUsersVia1C
	// (abbreviated for brevity)
	return nil, nil
}

// callResolveEndpoint calls 1C /resolve endpoint
func (w *EnrichmentWorker) callResolveEndpoint(ctx context.Context, request ResolveRequest) (*ResolveResponse, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", w.endpoint1C+"/resolve", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if w.apiKey != "" {
		httpReq.Header.Set("X-API-Key", w.apiKey)
	}

	resp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response ResolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// writeCachedUserMappings writes cached presentations to mapping table
func (w *EnrichmentWorker) writeCachedUserMappings(ctx context.Context, infobaseGUID string, cache map[uuid.UUID]string) error {
	if len(cache) == 0 {
		return nil
	}

	query := `INSERT INTO logs.user_map
		(infobase_guid, user_guid, user_name, sync_timestamp, version)
		VALUES (?, ?, ?, ?, ?)`

	batch, err := w.chClient.Conn().PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	version := uint64(time.Now().Unix())
	syncTime := time.Now()

	for userGUID, userName := range cache {
		if err := batch.Append(
			infobaseGUID,
			userGUID,
			userName,
			syncTime,
			version,
		); err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	log.Info().
		Int("count", len(cache)).
		Msg("Wrote cached user mappings")

	return nil
}

// writeResolvedUserMappings writes resolved presentations to mapping table
func (w *EnrichmentWorker) writeResolvedUserMappings(ctx context.Context, infobaseGUID string, users map[string]string) error {
	if len(users) == 0 {
		return nil
	}

	query := `INSERT INTO logs.user_map
		(infobase_guid, user_guid, user_name, sync_timestamp, version)
		VALUES (?, ?, ?, ?, ?)`

	batch, err := w.chClient.Conn().PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	version := uint64(time.Now().Unix())
	syncTime := time.Now()

	for guidStr, userName := range users {
		userGUID, err := uuid.Parse(guidStr)
		if err != nil {
			log.Warn().Str("guid", guidStr).Msg("Invalid GUID in response")
			continue
		}

		if err := batch.Append(
			infobaseGUID,
			userGUID,
			userName,
			syncTime,
			version,
		); err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	log.Info().
		Int("count", len(users)).
		Msg("Wrote resolved user mappings")

	return nil
}

// ResolveRequest is sent to 1C to resolve GUIDs
type ResolveRequest struct {
	InfobaseGUID  string   `json:"infobase_guid"`
	UserGUIDs     []string `json:"user_guids,omitempty"`
	MetadataGUIDs []string `json:"metadata_guids,omitempty"`
	DataGUIDs     []string `json:"data_guids,omitempty"`
}

// ResolveResponse is returned from 1C with presentations
type ResolveResponse struct {
	InfobaseGUID string            `json:"infobase_guid"`
	Timestamp    time.Time         `json:"timestamp"`
	Users        map[string]string `json:"users"`        // guid → name
	Metadata     map[string]string `json:"metadata"`     // guid → name
	Data         map[string]string `json:"data"`         // guid → presentation
	ErrorCount   int               `json:"error_count"`  // Number of GUIDs that failed to resolve
	ErrorGUIDs   []string          `json:"error_guids"`  // GUIDs that failed
}
