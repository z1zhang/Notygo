package processor

import (
	"context"
	"crypto/rand"
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
		workerID:     workerID,
	}
}

// Run starts the event processing loop. Blocks until context is cancelled.
func (p *EventProcessor) Run(ctx context.Context) {
	slog.Info("event processor starting", "worker_id", p.workerID, "concurrency", p.cfg.EventProcessorConcurrency)

	sem := make(chan struct{}, p.cfg.EventProcessorConcurrency)
	var wg sync.WaitGroup

	ticker := time.NewTicker(p.cfg.ClaimPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("event processor stopping, waiting for in-flight tasks", "worker_id", p.workerID)
			wg.Wait()
			slog.Info("event processor stopped", "worker_id", p.workerID)
			return

		case <-ticker.C:
			// Try to claim an event
			evt, err := p.eventRepo.ClaimEvent(ctx, p.workerID, p.cfg.LeaseTimeout)
			if err != nil {
				if err != event.ErrAlreadyClaimed {
					slog.Error("failed to claim event", "error", err)
				}
				continue
			}

			// Acquire semaphore slot
			select {
			case sem <- struct{}{}:
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					p.processEvent(ctx, evt)
				}()
			default:
				// At concurrency limit; release the claim back
				slog.Warn("event processor at concurrency limit, releasing claim", "event_id", evt.ID)
				_ = p.eventRepo.SetStatus(ctx, evt.ID, event.EventStatusReceived)
			}
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
			// Continue without mapping
		} else {
			mappingID = &mapObj.ID
			mappingRevision = &mapObj.Revision
			slog.Info("mapping loaded", "event_id", evt.ID, "mapping_id", mapObj.ID, "mapping_name", mapObj.Name, "field_count", len(mapObj.Fields))
		}
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
		slog.Info("mapping applied", "event_id", evt.ID, "parsed_fields", parsedFields, "validation_errors", validationErrors)
	} else {
		slog.Info("no mapping applied, parsed_fields empty", "event_id", evt.ID)
	}

	// 3. Get enabled rules and build rule_set_snapshot
	enabledRules, err := p.ruleRepo.ListEnabled(ctx)
	if err != nil {
		slog.Error("failed to list enabled rules", "event_id", evt.ID, "error", err)
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
		RunType:          "initial",
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
	var matchedRuleIDs []string
	for _, rule := range enabledRules {
		matched, err := evaluateRule(rule, parsedFields)
		if err != nil {
			slog.Warn("rule evaluation error", "rule_id", rule.ID, "rule_name", rule.Name, "error", err)
			continue
		}
		slog.Info("rule evaluated", "event_id", evt.ID, "rule_id", rule.ID, "rule_name", rule.Name, "matched", matched)
		if matched {
			matchedRuleIDs = append(matchedRuleIDs, rule.ID)

			// 6. For each matched rule: render template, create deliveries
			p.processMatchedRule(ctx, evt, createdRun, rule, parsedFields)
		}
	}

	// 7. Mark EventRun completed, Mark Event processed
	_ = p.eventRepo.CompleteEventRun(ctx, createdRun.ID, matchedRuleIDs)
	_ = p.eventRepo.MarkProcessed(ctx, evt.ID)

	slog.Info("event processed", "event_id", evt.ID, "matched_rules", len(matchedRuleIDs))
}

// processMatchedRule renders the template and creates deliveries for a matched rule.
func (p *EventProcessor) processMatchedRule(ctx context.Context, evt *event.Event, run *event.EventRun, rule rules.Rule, fields map[string]any) {
	var renderedTitle, renderedBody, renderedFormat string

	// Render template if rule has one
	if rule.TemplateID != nil && *rule.TemplateID != "" {
		tmpl, err := p.templateRepo.GetByID(ctx, *rule.TemplateID)
		if err != nil {
			slog.Error("failed to fetch template", "template_id", *rule.TemplateID, "error", err)
		} else {
			// Build template data: mapped fields at top level + Event metadata
			templateData := make(map[string]any, len(fields)+1)
			for k, v := range fields {
				templateData[k] = v
			}
			templateData["Event"] = map[string]any{
				"ID":         evt.ID,
				"ReceivedAt": evt.ReceivedAt.Format(time.RFC3339),
				"SourceName": srcName(evt.SourceID, ctx, p.sourceRepo),
			}
			msg, renderErr := templates.Render(tmpl, templateData)
			if renderErr != nil {
				slog.Error("failed to render template", "template_id", *rule.TemplateID, "error", renderErr)
			} else {
				renderedTitle = msg.Title
				renderedBody = msg.Body
				renderedFormat = msg.Format
			}
		}
	}

	// Create deliveries for each destination
	for _, destID := range rule.DestinationIDs {
		idempotencyKey := fmt.Sprintf("%s-%s-%s", evt.ID, run.ID, destID)

		d := &delivery.Delivery{
			EventID:        evt.ID,
			EventRunID:     run.ID,
			RuleID:         rule.ID,
			DestinationID:  destID,
			IdempotencyKey: idempotencyKey,
			Status:         delivery.DeliveryStatusPending,
			MaxAttempts:    p.cfg.DefaultMaxAttempts,
			NextAttemptAt:  time.Now().UTC(),
			RenderedTitle:  &renderedTitle,
			RenderedBody:   &renderedBody,
			RenderedFormat: &renderedFormat,
		}

		_, err := p.deliveryRepo.Create(ctx, d)
		if err != nil {
			slog.Error("failed to create delivery", "event_id", evt.ID, "destination_id", destID, "error", err)
		}
	}
}

// evaluateRule evaluates a rule against parsed fields.
func evaluateRule(rule rules.Rule, fields map[string]any) (bool, error) {
	return evaluateConditionGroup(rule.Conditions, rule.ConditionMode, fields)
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
		slog.Debug("condition field not found in parsed_fields", "field", cond.Field, "operator", cond.Operator, "value", cond.Value, "available_fields", formatMapKeys(fields))
		if cond.Operator == "exists" {
			return cond.Value == "false"
		}
		return false
	}

	fieldStr := fmt.Sprintf("%v", fieldVal)
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
		result = cond.Value != "false"
	case "not_exists":
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
		result = false
	}
	slog.Debug("condition evaluated", "field", cond.Field, "field_type", fmt.Sprintf("%T", fieldVal), "field_value", fieldStr, "operator", cond.Operator, "compare_value", cond.Value, "result", result)
	return result
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
	aStr := fmt.Sprintf("%v", a)
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

// srcName resolves a source ID to its name for template metadata.
func srcName(sourceID string, ctx context.Context, repo *source.Repository) string {
	src, err := repo.GetByID(ctx, sourceID)
	if err != nil {
		return sourceID
	}
	return src.Name
}

func generateWorkerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
