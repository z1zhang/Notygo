package processor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"zpush/internal/config"
	"zpush/internal/delivery"
	"zpush/internal/destination"
	"zpush/internal/event"
	"zpush/internal/mapping"
	"zpush/internal/notify"
	"zpush/internal/rules"
	"zpush/internal/source"
	"zpush/internal/templates"
)

// EventProcessor processes received events by claiming them, applying mappings,
// evaluating rules, rendering templates, and creating deliveries.
type EventProcessor struct {
	cfg          config.WorkerConfig
	sourceRepo   *source.Repository
	mappingRepo  *mapping.Repository
	ruleRepo     *rules.Repository
	templateRepo *templates.Repository
	destRepo     *destination.Repository
	eventRepo    *event.Repository
	deliveryRepo *delivery.Repository
	notifier     *notify.Notifier
	workerID     string
}

// NewEventProcessor creates a new EventProcessor.
func NewEventProcessor(
	cfg config.WorkerConfig,
	sourceRepo *source.Repository,
	mappingRepo *mapping.Repository,
	ruleRepo *rules.Repository,
	templateRepo *templates.Repository,
	destRepo *destination.Repository,
	eventRepo *event.Repository,
	deliveryRepo *delivery.Repository,
	notifier *notify.Notifier,
) *EventProcessor {
	workerID := generateWorkerID()
	return &EventProcessor{
		cfg:          cfg,
		sourceRepo:   sourceRepo,
		mappingRepo:  mappingRepo,
		ruleRepo:     ruleRepo,
		templateRepo: templateRepo,
		destRepo:     destRepo,
		eventRepo:    eventRepo,
		deliveryRepo: deliveryRepo,
		notifier:     notifier,
		workerID:     workerID,
	}
}

// Run starts the event processing loop. Blocks until context is cancelled.
func (p *EventProcessor) Run(ctx context.Context) {
	slog.Info("event processor starting", "worker_id", p.workerID, "concurrency", p.cfg.EventProcessorConcurrency)

	// Work channel: buffered to concurrency capacity. When full, the dispatcher
	// blocks on send, providing natural backpressure.
	workCh := make(chan *event.Event, p.cfg.EventProcessorConcurrency)

	// doneCh signals when a worker finishes processing — allows the dispatcher
	// to wake up immediately and claim more events without waiting for the ticker.
	doneCh := make(chan struct{}, p.cfg.EventProcessorConcurrency)

	// Start worker pool — N goroutines reading from workCh
	var wg sync.WaitGroup
	for i := 0; i < p.cfg.EventProcessorConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for evt := range workCh {
				func() {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("event processor worker panic", "event_id", evt.ID, "panic", r)
						}
					}()
					p.processEvent(ctx, evt)
				}()
				// Signal that a slot is free (non-blocking, doneCh is buffered)
				select {
				case doneCh <- struct{}{}:
				default:
				}
			}
		}()
	}

	// Dispatcher: wait for notification, worker-done, or ticker, then batch-claim loop
	ticker := time.NewTicker(p.cfg.ClaimPollInterval)
	defer ticker.Stop()

	notifyCh := p.notifier.EventCh() // nil if notifier is nil

dispatch:
	for {
		// Wait for a signal: notification, worker-done, or ticker fallback
		if notifyCh != nil {
			select {
			case <-ctx.Done():
				break dispatch
			case <-notifyCh:
				// New event arrived, drain coalesced signals
				drainChannel(notifyCh)
			case <-doneCh:
				// A worker finished, try to claim more immediately
			case <-ticker.C:
				// Fallback poll
			}
		} else {
			// No notifier: listen on doneCh + ticker
			select {
			case <-ctx.Done():
				break dispatch
			case <-doneCh:
			case <-ticker.C:
			}
		}

		// Batch claim loop: claim events until workCh is full or no more claimable
		for {
			select {
			case <-ctx.Done():
				break dispatch
			default:
			}

			evt, err := p.eventRepo.ClaimEvent(ctx, p.workerID, p.cfg.LeaseTimeout)
			if err != nil {
				if err != event.ErrAlreadyClaimed {
					// Transient DB error — log and retry after brief pause
					slog.Error("claim event error, retrying", "error", err)
					select {
					case <-ctx.Done():
						break dispatch
					case <-time.After(200 * time.Millisecond):
						continue // retry the claim immediately
					}
				}
				break // no more events to claim right now
			}

			// Push to workCh. This blocks when the channel is full, which means
			// all N workers are busy. This is natural backpressure — the dispatcher
			// stops claiming until a worker frees up. The claim is held under lease
			// so it won't be lost; the worker will process it when a slot opens.
			select {
			case workCh <- evt:
				// Event dispatched to worker pool
			case <-ctx.Done():
				// Context cancelled while waiting for a slot.
				// The event is claimed but not dispatched. Lease recovery will handle it.
				break dispatch
			}
		}
	}

	// Graceful shutdown: close workCh so workers drain, then wait
	close(workCh)
	slog.Info("event processor stopping, waiting for in-flight tasks", "worker_id", p.workerID)
	wg.Wait()
	slog.Info("event processor stopped", "worker_id", p.workerID)
}

// drainChannel drains any coalesced signals from a notification channel.
func drainChannel(ch <-chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// processEvent handles a single event through the full processing pipeline.
func (p *EventProcessor) processEvent(ctx context.Context, evt *event.Event) {
	slog.Info("processing event", "event_id", evt.ID, "source_id", evt.SourceID)

	// 1. Fetch source + mapping
	src, err := p.sourceRepo.GetByID(ctx, evt.SourceID)
	if err != nil {
		slog.Error("failed to fetch source for event", "event_id", evt.ID, "source_id", evt.SourceID, "error", err)
		_ = p.eventRepo.MarkFailed(ctx, evt.ID, "PROCESSOR_001", "source not found")
		return
	}

	var mapObj *mapping.Mapping
	var mappingID *string
	var mappingRevision *int

	if src.MappingID != nil && *src.MappingID != "" {
		mapObj, err = p.mappingRepo.GetByID(ctx, *src.MappingID)
		if err != nil {
			slog.Error("failed to fetch mapping for event", "event_id", evt.ID, "mapping_id", *src.MappingID, "error", err)
			_ = p.eventRepo.MarkFailed(ctx, evt.ID, "PROCESSOR_003", fmt.Sprintf("mapping %s not found", *src.MappingID))
			return
		}
		mappingID = &mapObj.ID
		mappingRevision = &mapObj.Revision
		slog.Info("mapping loaded", "event_id", evt.ID, "mapping_id", mapObj.ID, "mapping_name", mapObj.Name, "field_count", len(mapObj.Fields), "rule_ids", mapObj.RuleIDs)
	} else {
		slog.Warn("no mapping configured for source", "event_id", evt.ID, "source_id", src.ID, "source_name", src.Name)
	}

	// 2. Parse the mapping.Fields JSON into []MappingEntry and apply mapping
	parsedFields := map[string]any{}
	validationErrors := map[string]any{}

	if mapObj != nil {
		var valErrs []mapping.ValidationError
		parsedFields, valErrs, err = mapping.Apply(mapObj, evt.PayloadRaw)
		if err != nil {
			slog.Error("failed to apply mapping", "event_id", evt.ID, "error", err)
			validationErrors["mapping_error"] = err.Error()
		}
		if len(valErrs) > 0 {
			for _, ve := range valErrs {
				validationErrors[ve.Field] = ve.Detail
			}
		}
		slog.Info("mapping applied", "event_id", evt.ID, "parsed_fields", formatFieldSummary(parsedFields), "validation_errors", validationErrors)
	} else {
		slog.Info("no mapping applied, parsed_fields empty", "event_id", evt.ID)
	}

	// 2b. Build evalFields: parsedFields + system meta fields.
	// parsedFields stays clean (only business fields from the mapping) and is
	// stored in the EventRun. evalFields adds meta fields that rules and
	// dedupe can reference (e.g. key_fields: ["payload_hash"]).
	evalFields := make(map[string]any, len(parsedFields)+2)
	for k, v := range parsedFields {
		evalFields[k] = v
	}
	evalFields["payload_hash"] = evt.PayloadHash
	evalFields["source_id"] = evt.SourceID

	// 3. Get enabled rules selected by this mapping, and build rule_set_snapshot
	var enabledRules []rules.Rule
	if mapObj != nil && len(mapObj.RuleIDs) > 0 {
		enabledRules, err = p.ruleRepo.ListEnabledByIDs(ctx, mapObj.RuleIDs)
		if err != nil {
			slog.Error("failed to list enabled rules by mapping", "event_id", evt.ID, "mapping_id", mapObj.ID, "rule_ids", mapObj.RuleIDs, "error", err)
			_ = p.eventRepo.MarkFailed(ctx, evt.ID, "PROCESSOR_004", fmt.Sprintf("failed to fetch rules for mapping %s", mapObj.ID))
			return
		}
		slog.Info("rules fetched for event", "event_id", evt.ID, "requested_rule_ids", mapObj.RuleIDs, "fetched_count", len(enabledRules))
	} else {
		if mapObj != nil {
			slog.Warn("no rules configured for mapping — event will have zero matched rules", "event_id", evt.ID, "mapping_id", mapObj.ID, "mapping_name", mapObj.Name)
		}
		enabledRules = []rules.Rule{}
	}

	ruleSetSnapshot := make([]event.RuleSnapshot, 0, len(enabledRules))
	for _, r := range enabledRules {
		ruleSetSnapshot = append(ruleSetSnapshot, event.RuleSnapshot{
			ID:           r.ID,
			Name:         r.Name,
			Priority:     r.Priority,
			Conditions:   r.Conditions,
			Destinations: r.DestinationIDs,
		})
	}

	// 4. Create EventRun with all metadata
	eventRun := &event.EventRun{
		EventID:          evt.ID,
		RunType:          evt.RunType,
		MappingID:        mappingID,
		MappingRevision:  mappingRevision,
		RuleSetSnapshot:  ruleSetSnapshot,
		ParsedFields:     parsedFields,
		ValidationResult: validationErrors,
		MatchedRuleIDs:   []string{},
		Status:           event.EventRunStatusProcessing,
	}

	createdRun, err := p.eventRepo.CreateEventRun(ctx, eventRun)
	if err != nil {
		slog.Error("failed to create event run", "event_id", evt.ID, "error", err)
		_ = p.eventRepo.MarkFailed(ctx, evt.ID, "PROCESSOR_002", "failed to create event run")
		return
	}

	// 5. Evaluate rules
	continueOnMatch := mapObj != nil && mapObj.ContinueOnMatch
	var matchedRuleIDs []string
	deliveryCreated := false
	var renderFailure error
	for _, rule := range enabledRules {
		matched, err := evaluateRule(rule, evalFields)
		if err != nil {
			slog.Warn("rule evaluation error", "event_id", evt.ID, "rule_id", rule.ID, "rule_name", rule.Name, "error", err)
			continue
		}
		slog.Info("rule evaluated", "event_id", evt.ID, "rule_id", rule.ID, "rule_name", rule.Name, "matched", matched)
		if matched {
			matchedRuleIDs = append(matchedRuleIDs, rule.ID)

			// 6. For each matched rule: render template, create deliveries
			created, renderErr := p.processMatchedRule(ctx, evt, src, createdRun, rule, evalFields)
			if renderErr != nil {
				// Template render failure — do NOT create empty-content deliveries;
				// fail the event so the error is visible and fixable
				slog.Error("template render failed, skipping deliveries for rule",
					"event_id", evt.ID, "rule_id", rule.ID, "rule_name", rule.Name, "error", renderErr)
				if renderFailure == nil {
					renderFailure = renderErr
				}
			} else if created {
				deliveryCreated = true
			}

			// If continue_on_match is false (mapping-level), stop after the first match
			if !continueOnMatch {
				break
			}
		}
	}

	// 7. Mark EventRun completed (or failed when a template could not render)
	if renderFailure != nil {
		_ = p.eventRepo.FailEventRun(ctx, createdRun.ID, "PROCESSOR_005", renderFailure.Error())
		_ = p.eventRepo.MarkFailed(ctx, evt.ID, "PROCESSOR_005", renderFailure.Error())
		slog.Error("event failed: template render error", "event_id", evt.ID, "error", renderFailure)
		return
	}
	_ = p.eventRepo.CompleteEventRun(ctx, createdRun.ID, matchedRuleIDs)

	// 8. Mark Event status: deduped if all matched rules were skipped, completed otherwise
	if len(matchedRuleIDs) > 0 && !deliveryCreated {
		_ = p.eventRepo.MarkDeduped(ctx, evt.ID)
		slog.Info("event deduped", "event_id", evt.ID, "matched_rules", len(matchedRuleIDs), "all_deduped", true)
	} else {
		_ = p.eventRepo.MarkProcessed(ctx, evt.ID)
		slog.Info("event processed", "event_id", evt.ID, "matched_rules", len(matchedRuleIDs), "deliveries_created", deliveryCreated)
	}
	if len(matchedRuleIDs) == 0 && mapObj != nil {
		slog.Warn("event completed with zero matched rules",
			"event_id", evt.ID,
			"mapping_id", mapObj.ID,
			"mapping_name", mapObj.Name,
			"rules_evaluated", len(enabledRules),
			"available_fields", formatFieldSummary(evalFields),
		)
	}
}

// processMatchedRule renders the template and creates deliveries for a matched rule.
// If the rule has dedupe enabled, it checks for duplicates before creating deliveries.
// Returns (true, nil) if deliveries were actually created, (false, nil) if dedupe-skipped,
// and (_, err) when the template could not be rendered (no deliveries are created then).
func (p *EventProcessor) processMatchedRule(ctx context.Context, evt *event.Event, src *source.Source, run *event.EventRun, rule rules.Rule, fields map[string]any) (bool, error) {
	// === Deduplication check ===
	if rule.DedupeConfig != nil && rule.DedupeConfig.WindowSeconds > 0 {
		// Dedupe key: rule_id + payload_hash â same content within window = skip
		dedupeKey := rule.ID + ":payload_hash=" + evt.PayloadHash
		dedupeHash := sha256Hash(dedupeKey)

		result, err := p.deliveryRepo.CheckAndInsertDedupeKey(ctx, rule.ID, dedupeHash, rule.DedupeConfig.WindowSeconds, time.Now().UTC())
		if err != nil {
			// Dedupe check failed â degrade gracefully (prefer extra delivery over lost delivery)
			slog.Error("dedupe check failed, proceeding without dedupe", "event_id", evt.ID, "rule_id", rule.ID, "error", err)
		} else if result.IsDuplicate {
			slog.Info("dedupe: skipping duplicate event", "event_id", evt.ID, "rule_id", rule.ID, "payload_hash", evt.PayloadHash)
			return false, nil
		} else {
			slog.Info("dedupe: first occurrence in window", "event_id", evt.ID, "rule_id", rule.ID, "payload_hash", evt.PayloadHash)
		}
	}

	// Render template and create deliveries
	return true, p.renderAndCreateDeliveries(ctx, evt, src, run, rule, fields)
}

// renderAndCreateDeliveries renders the template and creates deliveries.
// Rendering failures return an error — deliveries with empty content are never created.
func (p *EventProcessor) renderAndCreateDeliveries(ctx context.Context, evt *event.Event, src *source.Source, run *event.EventRun, rule rules.Rule, fields map[string]any) error {
	if rule.TemplateID == nil || *rule.TemplateID == "" {
		return fmt.Errorf("rule %q has no template configured", rule.Name)
	}

	tmpl, err := p.templateRepo.GetByID(ctx, *rule.TemplateID)
	if err != nil {
		return fmt.Errorf("template %s not found: %w", *rule.TemplateID, err)
	}

	// Build template data: mapped fields at top level + Event metadata
	templateData := make(map[string]any, len(fields)+1)
	for k, v := range fields {
		templateData[k] = v
	}
	templateData["Event"] = map[string]any{
		"ID":         evt.ID,
		"ReceivedAt": evt.ReceivedAt.Format(time.RFC3339),
		"SourceName": src.Name,
	}
	msg, renderErr := templates.Render(tmpl, templateData)
	if renderErr != nil {
		return fmt.Errorf("template %s render failed: %w", *rule.TemplateID, renderErr)
	}

	renderedTitle := msg.Title
	renderedBody := msg.Body
	renderedFormat := msg.Format

	nextAttemptAt := time.Now().UTC()

	// Create deliveries for each destination. The retry budget comes from the
	// destination's max_attempts (fallback: DB default 5 for deleted/legacy rows).
	for _, destID := range rule.DestinationIDs {
		idempotencyKey := fmt.Sprintf("%s-%s-%s-%s", evt.ID, run.ID, rule.ID, destID)

		maxAttempts := 5
		if dest, err := p.destRepo.GetByID(ctx, destID); err == nil && dest.MaxAttempts > 0 {
			maxAttempts = dest.MaxAttempts
		}

		d := &delivery.Delivery{
			EventID:        evt.ID,
			EventRunID:     run.ID,
			RuleID:         rule.ID,
			DestinationID:  destID,
			IdempotencyKey: idempotencyKey,
			Status:         delivery.DeliveryStatusPending,
			MaxAttempts:    maxAttempts,
			NextAttemptAt:  nextAttemptAt,
			RenderedTitle:  &renderedTitle,
			RenderedBody:   &renderedBody,
			RenderedFormat: &renderedFormat,
		}

		_, err := p.deliveryRepo.Create(ctx, d)
		if err != nil {
			slog.Error("failed to create delivery", "event_id", evt.ID, "destination_id", destID, "error", err)
		}
	}

	// Notify the delivery worker that new deliveries are available
	if p.notifier != nil {
		p.notifier.NotifyDelivery()
	}

	return nil
}

// evaluateRule evaluates a rule against parsed fields.
func evaluateRule(rule rules.Rule, fields map[string]any) (bool, error) {
	return evaluateConditionGroup(rule.Conditions, rule.Conditions.Operator, fields)
}

// evaluateConditionGroup evaluates a condition group against parsed fields.
func evaluateConditionGroup(group rules.ConditionGroup, mode string, fields map[string]any) (bool, error) {
	if len(group.Conditions) == 0 && len(group.Groups) == 0 {
		return true, nil
	}

	// Evaluate sub-groups
	var groupResults []bool
	for _, g := range group.Groups {
		result, err := evaluateConditionGroup(g, g.Operator, fields)
		if err != nil {
			return false, err
		}
		groupResults = append(groupResults, result)
	}

	// Evaluate conditions
	var condResults []bool
	for _, cond := range group.Conditions {
		result := evaluateCondition(cond, fields)
		condResults = append(condResults, result)
	}

	// Combine results
	allResults := append(condResults, groupResults...)
	if len(allResults) == 0 {
		return true, nil
	}

	effectiveMode := mode
	if effectiveMode == "" {
		effectiveMode = "and"
	}

	switch effectiveMode {
	case "or":
		for _, r := range allResults {
			if r {
				return true, nil
			}
		}
		return false, nil
	default: // "and"
		for _, r := range allResults {
			if !r {
				return false, nil
			}
		}
		return true, nil
	}
}

// evaluateCondition evaluates a single condition against parsed fields.
func evaluateCondition(cond rules.Condition, fields map[string]any) bool {
	fieldVal, ok := fields[cond.Field]
	if !ok {
		slog.Info("condition field not found in parsed_fields", "field", cond.Field, "operator", cond.Operator, "value", cond.Value, "available_fields", formatMapKeys(fields))
		// Field does not exist in parsed_fields
		switch cond.Operator {
		case "exists":
			// "exists true" → field should exist → false (it doesn't)
			// "exists false" → field should not exist → true (it doesn't)
			return cond.Value == "false"
		case "not_exists":
			// "not_exists true" → field should not exist → true (it doesn't)
			// "not_exists false" → field should exist → false (it doesn't)
			return cond.Value != "false"
		default:
			// All other operators fail when the field doesn't exist
			return false
		}
	}

	fieldStr := formatFieldValue(fieldVal)
	var result bool
	switch cond.Operator {
	case "eq":
		result = fieldStr == cond.Value
	case "neq":
		result = fieldStr != cond.Value
	case "contains":
		result = strings.Contains(fieldStr, cond.Value)
	case "not_contains":
		result = !strings.Contains(fieldStr, cond.Value)
	case "exists":
		// Field exists: "exists true" → true, "exists false" → false
		result = cond.Value != "false"
	case "not_exists":
		// Field exists: "not_exists true" → false, "not_exists false" → true
		result = cond.Value == "false"
	case "gt", "lt", "gte", "lte":
		result = compareValues(fieldVal, cond.Value, cond.Operator)
	case "regex":
		if len(cond.Value) > 256 {
			slog.Warn("regex pattern exceeds 256 character limit", "pattern_length", len(cond.Value))
			result = false
		} else if re, err := regexp.Compile(cond.Value); err != nil {
			slog.Warn("invalid regex pattern", "pattern", cond.Value, "error", err)
			result = false
		} else {
			result = re.MatchString(fieldStr)
		}
	default:
		slog.Warn("unknown condition operator", "operator", cond.Operator, "field", cond.Field)
		result = false
	}
	slog.Info("condition evaluated", "field", cond.Field, "field_type", fmt.Sprintf("%T", fieldVal), "field_value", fieldStr, "operator", cond.Operator, "compare_value", cond.Value, "result", result)
	return result
}

// formatFieldValue converts a field value to string for comparison.
// Uses strconv for float64 to avoid scientific notation (e.g. 1e+09) that
// fmt.Sprintf("%v") can produce for large numbers, which would cause
// "eq" comparisons to fail against user-provided values like "1000000000".
func formatFieldValue(val any) string {
	switch v := val.(type) {
	case float64:
		// Format without scientific notation: 1000000000 not 1e+09
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%f", v)
	case float32:
		if v == float32(int32(v)) {
			return fmt.Sprintf("%d", int32(v))
		}
		return fmt.Sprintf("%f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatFieldSummary returns a concise summary of parsed fields for logging.
// Shows each key with its value type and truncated value.
func formatFieldSummary(fields map[string]any) map[string]string {
	summary := make(map[string]string, len(fields))
	for k, v := range fields {
		s := formatFieldValue(v)
		if len(s) > 80 {
			s = s[:77] + "..."
		}
		summary[k] = fmt.Sprintf("%s:%s", fmt.Sprintf("%T", v), s)
	}
	return summary
}

// formatMapKeys returns the keys of a map for logging purposes.
func formatMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func compareValues(a any, b string, op string) bool {
	aStr := formatFieldValue(a)
	var aNum, bNum float64
	_, err1 := fmt.Sscanf(aStr, "%f", &aNum)
	_, err2 := fmt.Sscanf(b, "%f", &bNum)
	if err1 == nil && err2 == nil {
		switch op {
		case "gt":
			return aNum > bNum
		case "lt":
			return aNum < bNum
		case "gte":
			return aNum >= bNum
		case "lte":
			return aNum <= bNum
		}
	}
	switch op {
	case "gt":
		return aStr > b
	case "lt":
		return aStr < b
	case "gte":
		return aStr >= b
	case "lte":
		return aStr <= b
	}
	return false
}

func generateWorkerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// sha256Hash returns the hex-encoded SHA-256 hash of the input string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
