package cache

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// HotDataWatcher watches for changes to hot data and updates the cache accordingly
type HotDataWatcher[T any] struct {
	collection    *mongo.Collection
	cache         Cache[T]
	accessTracker *AccessTracker
	ctx           context.Context
	cancel        context.CancelFunc
	watchInterval time.Duration
	decayInterval time.Duration
	mu            sync.RWMutex
	watching      map[primitive.ObjectID]bool
	logger        *zap.Logger
	options       *HotDataWatcherOptions

	// Change stream management
	streamMu     sync.Mutex
	changeStream *mongo.ChangeStream
	streamCtx    context.Context
	streamCancel context.CancelFunc

	// Performance monitoring
	perfMu               sync.Mutex
	eventProcessingTimes []time.Duration
	lastStrategyChange   time.Time
	currentStrategy      string
	eventCount           int
	filteredEventCount   int
	processingTimeTotal  time.Duration
	processingTimeAvg    time.Duration
}

// HotDataWatcherOptions contains options for the HotDataWatcher
type HotDataWatcherOptions struct {
	// MaxHotItems is the maximum number of hot items to track
	MaxHotItems int

	// DecayFactor is the factor to decay old access counts (0-1)
	DecayFactor float64

	// WatchInterval is how often to update the watch list
	WatchInterval time.Duration

	// DecayInterval is how often to decay scores
	DecayInterval time.Duration

	// Logger is the logger to use
	Logger *zap.Logger

	// AdaptiveMode determines whether to use adaptive monitoring based on document count
	AdaptiveMode bool

	// DocumentThreshold is the threshold for switching to collection-level monitoring
	// When the number of watched documents exceeds this threshold, we'll switch to
	// monitoring the entire collection and filtering on the client side
	DocumentThreshold int

	// MaxBatchSize is the maximum number of document IDs per batch
	// This is used when batching document IDs for the $in operator
	MaxBatchSize int

	// PerformanceMonitoringEnabled enables monitoring of change stream performance
	PerformanceMonitoringEnabled bool

	// PerformanceMonitoringInterval is how often to check change stream performance
	PerformanceMonitoringInterval time.Duration

	// MaxEventProcessingTime is the maximum time allowed for processing a change event
	// If events take longer than this to process, we'll switch to a more efficient strategy
	MaxEventProcessingTime time.Duration
}

// DefaultHotDataWatcherOptions returns the default options for HotDataWatcher
func DefaultHotDataWatcherOptions() *HotDataWatcherOptions {
	return &HotDataWatcherOptions{
		MaxHotItems:                   100,
		DecayFactor:                   0.95,
		WatchInterval:                 time.Minute * 5,
		DecayInterval:                 time.Hour,
		Logger:                        zap.NewNop(),
		AdaptiveMode:                  true,
		DocumentThreshold:             100, // Switch to collection-level monitoring when watching more than 100 docs
		MaxBatchSize:                  1000,
		PerformanceMonitoringEnabled:  true,
		PerformanceMonitoringInterval: time.Minute,
		MaxEventProcessingTime:        time.Millisecond * 100,
	}
}

// NewHotDataWatcher creates a new HotDataWatcher
func NewHotDataWatcher[T any](
	ctx context.Context,
	collection *mongo.Collection,
	cache Cache[T],
	opts *HotDataWatcherOptions,
) *HotDataWatcher[T] {
	if opts == nil {
		opts = DefaultHotDataWatcherOptions()
	}

	watcherCtx, cancel := context.WithCancel(ctx)

	// Create access tracker
	tracker := NewAccessTracker(opts.MaxHotItems, opts.DecayFactor)

	// Create watcher
	watcher := &HotDataWatcher[T]{
		collection:    collection,
		cache:         cache, // Use the generic cache directly
		accessTracker: tracker,
		ctx:           watcherCtx,
		cancel:        cancel,
		watchInterval: opts.WatchInterval,
		decayInterval: opts.DecayInterval,
		watching:      make(map[primitive.ObjectID]bool),
		logger:        opts.Logger,
		options:       opts,
	}

	// Initialize change stream context (will be used when starting the stream)
	streamCtx, streamCancel := context.WithCancel(watcherCtx)
	watcher.streamCtx = streamCtx
	watcher.streamCancel = streamCancel

	// Initialize performance monitoring
	watcher.eventProcessingTimes = make([]time.Duration, 0, 100)
	watcher.lastStrategyChange = time.Now()
	watcher.currentStrategy = "document-specific" // Start with document-specific filtering

	// Start background tasks
	go watcher.watchLoop()
	go watcher.decayLoop()
	go watcher.streamLoop() // Start the change stream processing loop

	// Start performance monitoring if enabled
	if opts.PerformanceMonitoringEnabled {
		go watcher.performanceMonitorLoop()
	}

	return watcher
}

// RecordAccess records an access to a document
func (w *HotDataWatcher[T]) RecordAccess(id primitive.ObjectID) {
	w.accessTracker.RecordAccess(id)
}

// IsWatching checks if a document is being watched
func (w *HotDataWatcher[T]) IsWatching(id primitive.ObjectID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.watching[id]
}

// GetHotItems returns the current hot items
func (w *HotDataWatcher[T]) GetHotItems() []primitive.ObjectID {
	return w.accessTracker.GetHotItems()
}

// Close stops the watcher
func (w *HotDataWatcher[T]) Close() {
	// Cancel main context (this will terminate all goroutines)
	w.cancel()

	// Close change stream if it exists
	w.streamMu.Lock()
	if w.changeStream != nil {
		w.logger.Debug("Closing change stream")
		w.streamCancel()
		w.changeStream.Close(context.Background())
		w.changeStream = nil
	}
	w.streamMu.Unlock()
}

// watchLoop periodically updates the watch list
func (w *HotDataWatcher[T]) watchLoop() {
	ticker := time.NewTicker(w.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.updateWatchList()
		case <-w.ctx.Done():
			return
		}
	}
}

// decayLoop periodically decays access scores
func (w *HotDataWatcher[T]) decayLoop() {
	ticker := time.NewTicker(w.decayInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.accessTracker.DecayScores()
		case <-w.ctx.Done():
			return
		}
	}
}

// updateWatchList updates the list of documents being watched
func (w *HotDataWatcher[T]) updateWatchList() {
	// Get current hot items
	hotItems := w.accessTracker.GetHotItems()

	w.mu.Lock()

	// Check if the watch list has changed
	changed := false
	if len(hotItems) != len(w.watching) {
		changed = true
	} else {
		// Check if any items are different
		for _, id := range hotItems {
			if !w.watching[id] {
				changed = true
				break
			}
		}
	}

	// Create a new map for the updated watch list
	newWatching := make(map[primitive.ObjectID]bool, len(hotItems))

	// Add all hot items to the new watch list
	for _, id := range hotItems {
		newWatching[id] = true
	}

	// Update the watch list
	w.watching = newWatching
	w.mu.Unlock()

	// If the watch list has changed, restart the change stream
	if changed {
		w.logger.Debug("Watch list changed, restarting change stream",
			zap.Int("item_count", len(hotItems)))
		w.restartChangeStream()
	}
}

// restartChangeStream restarts the change stream with the current watch list
func (w *HotDataWatcher[T]) restartChangeStream() {
	w.streamMu.Lock()
	defer w.streamMu.Unlock()

	// Close existing stream if any
	if w.changeStream != nil {
		w.logger.Debug("Closing existing change stream")
		w.streamCancel()
		w.changeStream.Close(context.Background())
		w.changeStream = nil
	}

	// Create a new context for the stream
	streamCtx, streamCancel := context.WithCancel(w.ctx)
	w.streamCtx = streamCtx
	w.streamCancel = streamCancel

	// Get the current watch list
	w.mu.RLock()
	watchCount := len(w.watching)
	if watchCount == 0 {
		w.mu.RUnlock()
		w.logger.Debug("No documents to watch, not starting change stream")
		return
	}

	// Create a list of document IDs to watch
	var docIDs []primitive.ObjectID
	for id := range w.watching {
		docIDs = append(docIDs, id)
	}
	w.mu.RUnlock()

	// Create a pipeline based on the number of documents we're watching
	var pipeline mongo.Pipeline

	// Check if we should use document-specific filtering or collection-level monitoring
	useDocumentFiltering := true

	// If adaptive mode is enabled and we're watching more documents than the threshold,
	// switch to collection-level monitoring
	if w.options.AdaptiveMode {
		if watchCount > w.options.DocumentThreshold {
			useDocumentFiltering = false
			w.logger.Info("Switching to collection-level monitoring due to large document count",
				zap.Int("document_count", watchCount),
				zap.Int("threshold", w.options.DocumentThreshold))
		}
	}

	// MongoDB has a limit on the size of the $in operator (usually around 16MB)
	// For very large document sets, we need to be careful
	maxBatchSize := w.options.MaxBatchSize

	if useDocumentFiltering && watchCount <= maxBatchSize {
		// For a small number of documents, use the $in operator for server-side filtering
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{
				{Key: "documentKey._id", Value: bson.D{
					{Key: "$in", Value: docIDs},
				}},
				{Key: "operationType", Value: bson.D{
					{Key: "$in", Value: bson.A{"insert", "update", "replace", "delete"}},
				}},
			}}},
		}
		w.currentStrategy = "document-specific"
		w.logger.Debug("Using document ID filtering in pipeline",
			zap.Int("document_count", watchCount))
	} else if useDocumentFiltering && watchCount > maxBatchSize {
		// For a medium number of documents, use the $or operator with multiple $in clauses
		// to avoid hitting the BSON document size limit

		// Calculate how many batches we need
		numBatches := (watchCount + maxBatchSize - 1) / maxBatchSize // Ceiling division

		// Create a $or array with multiple $in clauses
		orClauses := make([]bson.D, 0, numBatches)

		for i := 0; i < numBatches; i++ {
			start := i * maxBatchSize
			end := (i + 1) * maxBatchSize
			if end > watchCount {
				end = watchCount
			}

			batchIDs := docIDs[start:end]
			orClauses = append(orClauses, bson.D{
				{Key: "documentKey._id", Value: bson.D{
					{Key: "$in", Value: batchIDs},
				}},
			})
		}

		// Create the pipeline with the $or operator
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{
				{Key: "$and", Value: []bson.D{
					{{Key: "$or", Value: orClauses}},
					{{Key: "operationType", Value: bson.D{
						{Key: "$in", Value: bson.A{"insert", "update", "replace", "delete"}},
					}}},
				}},
			}}},
		}

		w.currentStrategy = "batched-document"
		w.logger.Debug("Using batched document ID filtering in pipeline",
			zap.Int("document_count", watchCount),
			zap.Int("batch_size", maxBatchSize),
			zap.Int("num_batches", numBatches))
	} else {
		// For a very large number of documents, only filter by operation type
		// and do document filtering on the client side
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{
				{Key: "operationType", Value: bson.D{
					{Key: "$in", Value: bson.A{"insert", "update", "replace", "delete"}},
				}},
			}}},
		}
		w.currentStrategy = "collection-level"
		w.logger.Debug("Using collection-level monitoring with client-side filtering",
			zap.Int("document_count", watchCount))
	}

	// Set up change stream options
	streamOpts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	// Create change stream
	stream, err := w.collection.Watch(streamCtx, pipeline, streamOpts)
	if err != nil {
		w.logger.Error("Failed to create change stream",
			zap.Error(err),
			zap.Int("document_count", watchCount))
		return
	}

	w.changeStream = stream
	w.logger.Debug("Started change stream",
		zap.Int("document_count", watchCount))
}

// streamLoop processes events from the change stream
func (w *HotDataWatcher[T]) streamLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			// Process change stream if it exists
			w.processChangeStream()

			// Sleep briefly to avoid tight loop if stream is nil
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// processChangeStream processes events from the current change stream
func (w *HotDataWatcher[T]) processChangeStream() {
	w.streamMu.Lock()
	stream := w.changeStream
	streamCtx := w.streamCtx
	w.streamMu.Unlock()

	// If no stream, return
	if stream == nil {
		return
	}

	// Process events until context is cancelled or error occurs
	for stream.Next(streamCtx) {
		// Start measuring processing time
		startTime := time.Now()

		// Decode the change event
		var event struct {
			OperationType string `bson:"operationType"`
			DocumentKey   struct {
				ID primitive.ObjectID `bson:"_id"`
			} `bson:"documentKey"`
			FullDocument bson.Raw `bson:"fullDocument"`
		}

		if err := stream.Decode(&event); err != nil {
			w.logger.Error("Failed to decode change event",
				zap.Error(err))
			continue
		}

		// Update event count for performance monitoring
		w.perfMu.Lock()
		w.eventCount++
		w.perfMu.Unlock()

		// Check if we're watching this document (client-side filtering)
		w.mu.RLock()
		watching := w.watching[event.DocumentKey.ID]
		w.mu.RUnlock()

		if !watching {
			// Skip events for documents we're not watching
			// This is more efficient than including all IDs in the pipeline
			continue
		}

		// Update filtered event count for performance monitoring
		w.perfMu.Lock()
		w.filteredEventCount++
		w.perfMu.Unlock()

		// Handle the event based on operation type
		switch event.OperationType {
		case "insert", "update", "replace":
			// Update cache with the new document
			if len(event.FullDocument) > 0 {
				// Unmarshal the document into the correct type
				var doc T
				if err := bson.Unmarshal(event.FullDocument, &doc); err != nil {
					w.logger.Error("Failed to unmarshal document",
						zap.Error(err),
						zap.String("document_id", event.DocumentKey.ID.Hex()),
						zap.String("operation", event.OperationType))
					continue
				}

				err := w.cache.Set(w.ctx, event.DocumentKey.ID.Hex(), doc, 0)
				if err != nil {
					w.logger.Error("Failed to update cache",
						zap.Error(err),
						zap.String("document_id", event.DocumentKey.ID.Hex()),
						zap.String("operation", event.OperationType))
				} else {
					w.logger.Debug("Updated cache from change stream",
						zap.String("document_id", event.DocumentKey.ID.Hex()),
						zap.String("operation", event.OperationType))
				}
			}
		case "delete":
			// Remove from cache
			err := w.cache.Delete(w.ctx, event.DocumentKey.ID.Hex())
			if err != nil {
				w.logger.Error("Failed to delete from cache",
					zap.Error(err),
					zap.String("document_id", event.DocumentKey.ID.Hex()))
			} else {
				w.logger.Debug("Deleted from cache due to change stream",
					zap.String("document_id", event.DocumentKey.ID.Hex()))
			}
		}

		// Record processing time for performance monitoring
		processingTime := time.Since(startTime)
		w.recordProcessingTime(processingTime)
	}

	// Check for errors
	if err := stream.Err(); err != nil {
		w.logger.Error("Change stream error, will restart",
			zap.Error(err))

		// Restart the stream
		w.restartChangeStream()
	}
}

// recordProcessingTime records the time taken to process an event
func (w *HotDataWatcher[T]) recordProcessingTime(duration time.Duration) {
	w.perfMu.Lock()
	defer w.perfMu.Unlock()

	// Add to processing times list (keep last 100 times)
	w.eventProcessingTimes = append(w.eventProcessingTimes, duration)
	if len(w.eventProcessingTimes) > 100 {
		w.eventProcessingTimes = w.eventProcessingTimes[1:]
	}

	// Update total and average
	w.processingTimeTotal += duration
	w.processingTimeAvg = w.processingTimeTotal / time.Duration(len(w.eventProcessingTimes))
}

// performanceMonitorLoop periodically checks performance metrics and adjusts strategy if needed
func (w *HotDataWatcher[T]) performanceMonitorLoop() {
	ticker := time.NewTicker(w.options.PerformanceMonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.checkPerformance()
		case <-w.ctx.Done():
			return
		}
	}
}

// checkPerformance analyzes performance metrics and adjusts the monitoring strategy if needed
func (w *HotDataWatcher[T]) checkPerformance() {
	w.perfMu.Lock()

	// Skip if we don't have enough data yet
	if len(w.eventProcessingTimes) < 10 {
		w.perfMu.Unlock()
		return
	}

	// Calculate metrics
	avgProcessingTime := w.processingTimeAvg
	totalEvents := w.eventCount
	filteredEvents := w.filteredEventCount
	filterRatio := 0.0
	if totalEvents > 0 {
		filterRatio = float64(filteredEvents) / float64(totalEvents)
	}

	// Reset counters for next period
	w.eventCount = 0
	w.filteredEventCount = 0

	w.perfMu.Unlock()

	// Log performance metrics
	w.logger.Debug("Change stream performance metrics",
		zap.Duration("avg_processing_time", avgProcessingTime),
		zap.Int("total_events", totalEvents),
		zap.Int("filtered_events", filteredEvents),
		zap.Float64("filter_ratio", filterRatio),
		zap.String("current_strategy", w.currentStrategy))

	// Check if we need to adjust the strategy
	needsAdjustment := false

	// If processing time is too high, consider switching to a more efficient strategy
	if avgProcessingTime > w.options.MaxEventProcessingTime {
		w.logger.Warn("Event processing time exceeds threshold, considering strategy adjustment",
			zap.Duration("avg_processing_time", avgProcessingTime),
			zap.Duration("threshold", w.options.MaxEventProcessingTime))
		needsAdjustment = true
	}

	// If filter ratio is very low (lots of events being filtered out), consider switching strategy
	if filterRatio < 0.1 && totalEvents > 100 {
		w.logger.Warn("Very low filter ratio, considering strategy adjustment",
			zap.Float64("filter_ratio", filterRatio),
			zap.Int("total_events", totalEvents),
			zap.Int("filtered_events", filteredEvents))
		needsAdjustment = true
	}

	// If we need to adjust and enough time has passed since the last adjustment
	if needsAdjustment && time.Since(w.lastStrategyChange) > time.Minute*5 {
		w.logger.Info("Adjusting change stream strategy based on performance metrics")

		// Get current watch list size
		w.mu.RLock()
		watchCount := len(w.watching)
		w.mu.RUnlock()

		// Adjust document threshold based on performance
		newThreshold := w.options.DocumentThreshold

		if avgProcessingTime > w.options.MaxEventProcessingTime*2 {
			// If processing is very slow, reduce threshold significantly
			newThreshold = newThreshold / 2
		} else if filterRatio < 0.05 {
			// If filter ratio is extremely low, reduce threshold
			newThreshold = newThreshold / 2
		} else if avgProcessingTime < w.options.MaxEventProcessingTime/2 && filterRatio > 0.5 {
			// If processing is fast and filter ratio is good, increase threshold
			newThreshold = newThreshold * 2
		}

		// Ensure threshold is within reasonable bounds
		if newThreshold < 10 {
			newThreshold = 10
		} else if newThreshold > 1000 {
			newThreshold = 1000
		}

		// Update threshold
		w.options.DocumentThreshold = newThreshold

		w.logger.Info("Adjusted document threshold based on performance",
			zap.Int("old_threshold", w.options.DocumentThreshold),
			zap.Int("new_threshold", newThreshold),
			zap.Int("current_watch_count", watchCount))

		// Restart change stream with new settings
		w.lastStrategyChange = time.Now()
		w.restartChangeStream()
	}
}
