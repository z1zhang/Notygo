// Package processor drives the event pipeline: applying mappings, evaluating
// rules, rendering templates, and creating deliveries. It runs inside the
// River event_process worker, one transaction per event.
package processor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notygo/internal/channel"
	"notygo/internal/delivery"
	"notygo/internal/endpoint"
	"notygo/internal/event"
	"notygo/internal/mapping"
	"notygo/internal/rules"
	"notygo/internal/templates"
)

// DeliveryEnqueuer queues a delivery_send job in the caller's transaction.
// Implemented by the queue manager; an interface here keeps processor
// independent of the queue package.
type DeliveryEnqueuer interface {
	EnqueueDeliverySendTx(ctx context.Context, tx pgx.Tx, deliveryID string, maxAttempts int) error
}

// Service processes events through the full pipeline.
type Service struct {
	pool         *pgxpool.Pool
	endpointRepo *endpoint.Repository
	mappingRepo  *mapping.Repository
	ruleRepo     *rules.Repository
	templateRepo *templates.Repository
	destRepo     *channel.Repository
	eventRepo    *event.Repository
	deliveryRepo *delivery.Repository
	enqueue      DeliveryEnqueuer
}

// NewService creates a processor Service. enqueue may be nil at construction
// and bound later with SetEnqueuer (the queue manager needs the processor to
// build its workers, and the processor needs the manager to enqueue — the
// reference is completed before the queue starts).
func NewService(
	pool *pgxpool.Pool,
	endpointRepo *endpoint.Repository,
	mappingRepo *mapping.Repository,
	ruleRepo *rules.Repository,
	templateRepo *templates.Repository,
	destRepo *channel.Repository,
	eventRepo *event.Repository,
	deliveryRepo *delivery.Repository,
	enqueue DeliveryEnqueuer,
) *Service {
	return &Service{
		pool:         pool,
		endpointRepo: endpointRepo,
		mappingRepo:  mappingRepo,
		ruleRepo:     ruleRepo,
		templateRepo: templateRepo,
		destRepo:     destRepo,
		eventRepo:    eventRepo,
		deliveryRepo: deliveryRepo,
		enqueue:      enqueue,
	}
}

// SetEnqueuer binds the delivery job enqueuer (two-phase construction).
func (s *Service) SetEnqueuer(enqueue DeliveryEnqueuer) {
	s.enqueue = enqueue
}

// Process handles a single event through the full processing pipeline in one
// transaction: the EventRun, matched-rule deliveries, their queue jobs, and
// the final event status are committed atomically. Business failures (missing
// endpoint/mapping, template render error) are written as status=failed and
// return nil — the queue job completes. An infrastructure error rolls the
// whole transaction back (the event stays `received`) and is returned so the
// caller can surface it; the stuck sweep re-drives such events later.
func (s *Service) Process(ctx context.Context, evt *event.Event) error {
	slog.Info("processing event", "event_id", evt.ID, "endpoint_id", evt.EndpointID)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after commit

	eventTx := s.eventRepo.WithTx(tx)
	deliveryTx := s.deliveryRepo.WithTx(tx)

	// Claim guard: flip received → processing. Zero rows means another job
	// already owns this event (duplicate enqueue) — nothing to do.
	claimed, err := eventTx.MarkProcessing(ctx, evt.ID)
	if err != nil {
		return fmt.Errorf("mark event processing: %w", err)
	}
	if !claimed {
		return nil
	}

	err = s.processEvent(ctx, tx, eventTx, deliveryTx, evt)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// processEvent handles a single event through the full processing pipeline
// against the given transaction. Business failures are recorded as
// status=failed and swallowed (nil); infrastructure errors are returned and
// roll back everything the run wrote.
func (s *Service) processEvent(ctx context.Context, tx pgx.Tx, eventTx *event.Repository, deliveryTx *delivery.Repository, evt *event.Event) error {
	// 1. Fetch endpoint + mapping
	src, err := s.endpointRepo.GetByID(ctx, evt.EndpointID)
	if err != nil {
		slog.Error("failed to fetch endpoint for event", "event_id", evt.ID, "endpoint_id", evt.EndpointID, "error", err)
		_ = eventTx.MarkFailed(ctx, evt.ID, "PROCESSOR_001", "endpoint not found")
		return nil
	}

	var mapObj *mapping.Mapping
	var mappingID *string
	var mappingRevision *int

	if src.MappingID != nil && *src.MappingID != "" {
		mapObj, err = s.mappingRepo.GetByID(ctx, *src.MappingID)
		if err != nil {
			slog.Error("failed to fetch mapping for event", "event_id", evt.ID, "mapping_id", *src.MappingID, "error", err)
			_ = eventTx.MarkFailed(ctx, evt.ID, "PROCESSOR_003", fmt.Sprintf("mapping %s not found", *src.MappingID))
			return nil
		}
		mappingID = &mapObj.ID
		mappingRevision = &mapObj.Revision
		slog.Info("mapping loaded", "event_id", evt.ID, "mapping_id", mapObj.ID, "mapping_name", mapObj.Name, "field_count", len(mapObj.Fields), "rule_ids", mapObj.RuleIDs)
	} else {
		slog.Warn("no mapping configured for endpoint", "event_id", evt.ID, "endpoint_id", src.ID, "endpoint_name", src.Name)
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
	evalFields["endpoint_id"] = evt.EndpointID

	// 3. Get enabled rules selected by this mapping, and build rule_set_snapshot
	var enabledRules []rules.Rule
	if mapObj != nil && len(mapObj.RuleIDs) > 0 {
		enabledRules, err = s.ruleRepo.ListEnabledByIDs(ctx, mapObj.RuleIDs)
		if err != nil {
			slog.Error("failed to list enabled rules by mapping", "event_id", evt.ID, "mapping_id", mapObj.ID, "rule_ids", mapObj.RuleIDs, "error", err)
			_ = eventTx.MarkFailed(ctx, evt.ID, "PROCESSOR_004", fmt.Sprintf("failed to fetch rules for mapping %s", mapObj.ID))
			return nil
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
			ID:         r.ID,
			Name:       r.Name,
			Priority:   r.Priority,
			Conditions: r.Conditions,
			Channels:   r.ChannelIDs,
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

	createdRun, err := eventTx.CreateEventRun(ctx, eventRun)
	if err != nil {
		return fmt.Errorf("create event run: %w", err)
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
			created, renderErr := s.processMatchedRule(ctx, tx, eventTx, deliveryTx, evt, src, createdRun, rule, evalFields)
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
		_ = eventTx.FailEventRun(ctx, createdRun.ID, "PROCESSOR_005", renderFailure.Error())
		_ = eventTx.MarkFailed(ctx, evt.ID, "PROCESSOR_005", renderFailure.Error())
		slog.Error("event failed: template render error", "event_id", evt.ID, "error", renderFailure)
		return nil
	}
	_ = eventTx.CompleteEventRun(ctx, createdRun.ID, matchedRuleIDs)

	// 8. Mark Event status: deduped if all matched rules were skipped, completed otherwise
	if len(matchedRuleIDs) > 0 && !deliveryCreated {
		_ = eventTx.MarkDeduped(ctx, evt.ID)
		slog.Info("event deduped", "event_id", evt.ID, "matched_rules", len(matchedRuleIDs), "all_deduped", true)
	} else {
		_ = eventTx.MarkProcessed(ctx, evt.ID)
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
	return nil
}

// processMatchedRule renders the template and creates deliveries for a matched rule.
// If the rule has dedupe enabled, it checks for duplicates before creating deliveries.
// Returns (true, nil) if deliveries were actually created, (false, nil) if dedupe-skipped,
// and (_, err) when the template could not be rendered (no deliveries are created then).
func (s *Service) processMatchedRule(ctx context.Context, tx pgx.Tx, eventTx *event.Repository, deliveryTx *delivery.Repository, evt *event.Event, src *endpoint.Endpoint, run *event.EventRun, rule rules.Rule, fields map[string]any) (bool, error) {
	// === Deduplication check ===
	if rule.DedupeConfig != nil && rule.DedupeConfig.WindowSeconds > 0 {
		// Dedupe key: rule_id + payload_hash — same content within window = skip
		dedupeKey := rule.ID + ":payload_hash=" + evt.PayloadHash
		dedupeHash := sha256Hash(dedupeKey)

		result, err := deliveryTx.CheckAndInsertDedupeKey(ctx, rule.ID, dedupeHash, rule.DedupeConfig.WindowSeconds, time.Now().UTC())
		if err != nil {
			// Dedupe check failed — degrade gracefully (prefer extra delivery over lost delivery)
			slog.Error("dedupe check failed, proceeding without dedupe", "event_id", evt.ID, "rule_id", rule.ID, "error", err)
		} else if result.IsDuplicate {
			slog.Info("dedupe: skipping duplicate event", "event_id", evt.ID, "rule_id", rule.ID, "payload_hash", evt.PayloadHash)
			return false, nil
		} else {
			slog.Info("dedupe: first occurrence in window", "event_id", evt.ID, "rule_id", rule.ID, "payload_hash", evt.PayloadHash)
		}
	}

	// Render template and create deliveries
	return true, s.renderAndCreateDeliveries(ctx, tx, deliveryTx, evt, src, run, rule, fields)
}

// renderAndCreateDeliveries renders the template and creates deliveries,
// queueing a delivery_send job for each inside the same transaction.
// Rendering failures return an error — deliveries with empty content are never created.
func (s *Service) renderAndCreateDeliveries(ctx context.Context, tx pgx.Tx, deliveryTx *delivery.Repository, evt *event.Event, src *endpoint.Endpoint, run *event.EventRun, rule rules.Rule, fields map[string]any) error {
	if rule.TemplateID == nil || *rule.TemplateID == "" {
		return fmt.Errorf("rule %q has no template configured", rule.Name)
	}

	tmpl, err := s.templateRepo.GetByID(ctx, *rule.TemplateID)
	if err != nil {
		return fmt.Errorf("template %s not found: %w", *rule.TemplateID, err)
	}

	// Build template data: mapped fields at top level + Event metadata
	templateData := make(map[string]any, len(fields)+1)
	for k, v := range fields {
		templateData[k] = v
	}
	templateData["Event"] = map[string]any{
		"ID":           evt.ID,
		"ReceivedAt":   evt.ReceivedAt.Format(time.RFC3339),
		"EndpointName": src.Name,
	}
	msg, renderErr := templates.Render(tmpl, templateData)
	if renderErr != nil {
		return fmt.Errorf("template %s render failed: %w", *rule.TemplateID, renderErr)
	}

	renderedTitle := msg.Title
	renderedBody := msg.Body
	renderedFormat := msg.Format

	// Create deliveries for each channel. The retry budget comes from the
	// channel's max_attempts (fallback: DB default 5 for deleted/legacy rows).
	for _, destID := range rule.ChannelIDs {
		maxAttempts := 5
		if dest, err := s.destRepo.GetByID(ctx, destID); err == nil && dest.MaxAttempts > 0 {
			maxAttempts = dest.MaxAttempts
		}

		idempotencyKey := fmt.Sprintf("%s-%s-%s-%s", evt.ID, run.ID, rule.ID, destID)
		d := &delivery.Delivery{
			EventID:        evt.ID,
			EventRunID:     run.ID,
			RuleID:         rule.ID,
			ChannelID:      destID,
			IdempotencyKey: idempotencyKey,
			Status:         delivery.DeliveryStatusPending,
			MaxAttempts:    maxAttempts,
			RenderedTitle:  &renderedTitle,
			RenderedBody:   &renderedBody,
			RenderedFormat: &renderedFormat,
		}

		created, err := deliveryTx.Create(ctx, d)
		if err != nil {
			return fmt.Errorf("create delivery: %w", err)
		}

		// Queue the send job in the same transaction — the delivery row and
		// its job become atomic: no row without a job, no job without a row.
		if err := s.enqueue.EnqueueDeliverySendTx(ctx, tx, created.ID, maxAttempts); err != nil {
			return fmt.Errorf("enqueue delivery job: %w", err)
		}
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

// sha256Hash returns the hex-encoded SHA-256 hash of the input string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
