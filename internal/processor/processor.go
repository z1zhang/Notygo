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
	"maps"
	"notygo/internal/errcode"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notygo/internal/channel"
	"notygo/internal/delivery"
	"notygo/internal/endpoint"
	"notygo/internal/event"
	"notygo/internal/mapping"
	"notygo/internal/rule"
	"notygo/internal/template"
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
	ruleRepo     *rule.Repository
	templateRepo *template.Repository
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
	ruleRepo *rule.Repository,
	templateRepo *template.Repository,
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
// markFailed, markDeduped, etc. return errors that this file deliberately
// swallows: the event's state transition failed, but the pipeline continues
// so a broken event cannot wedge the queue. The event stays in an
// inconsistent status with no DB record of why, so a log line is the only
// trace — log it instead of dropping the error on the floor.
func logStateTransition(eventID, action string, err error) {
	slog.Error("event state transition failed", "event_id", eventID, "action", action, "error", err)
}

func (s *Service) processEvent(ctx context.Context, tx pgx.Tx, eventTx *event.Repository, deliveryTx *delivery.Repository, evt *event.Event) error {
	// 1. Fetch endpoint + mapping
	src, err := s.endpointRepo.GetByID(ctx, evt.EndpointID)
	if err != nil {
		slog.Error("failed to fetch endpoint for event", "event_id", evt.ID, "endpoint_id", evt.EndpointID, "error", err)
		if err := eventTx.MarkFailed(ctx, evt.ID, errcode.ProcessorEndpointNotFound, "endpoint not found"); err != nil {
			logStateTransition(evt.ID, "mark_failed", err)
		}
		return nil
	}

	var mapObj *mapping.Mapping
	var mappingID *string
	var mappingRevision *int

	if src.MappingID != nil && *src.MappingID != "" {
		mapObj, err = s.mappingRepo.GetByID(ctx, *src.MappingID)
		if err != nil {
			slog.Error("failed to fetch mapping for event", "event_id", evt.ID, "mapping_id", *src.MappingID, "error", err)
			if err := eventTx.MarkFailed(ctx, evt.ID, errcode.ProcessorMappingNotFound, fmt.Sprintf("mapping %s not found", *src.MappingID)); err != nil {
				logStateTransition(evt.ID, "mark_failed", err)
			}
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
	maps.Copy(evalFields, parsedFields)
	evalFields["payload_hash"] = evt.PayloadHash
	evalFields["endpoint_id"] = evt.EndpointID

	// 3. Get enabled rules selected by this mapping, and build rule_set_snapshot
	var enabledRules []rule.Rule
	if mapObj != nil && len(mapObj.RuleIDs) > 0 {
		enabledRules, err = s.ruleRepo.ListEnabledByIDs(ctx, mapObj.RuleIDs)
		if err != nil {
			slog.Error("failed to list enabled rules by mapping", "event_id", evt.ID, "mapping_id", mapObj.ID, "rule_ids", mapObj.RuleIDs, "error", err)
			if err := eventTx.MarkFailed(ctx, evt.ID, errcode.ProcessorRulesFetchFailed, fmt.Sprintf("failed to fetch rules for mapping %s", mapObj.ID)); err != nil {
				logStateTransition(evt.ID, "mark_failed", err)
			}
			return nil
		}
		slog.Info("rules fetched for event", "event_id", evt.ID, "requested_rule_ids", mapObj.RuleIDs, "fetched_count", len(enabledRules))
	} else {
		if mapObj != nil {
			slog.Warn("no rules configured for mapping — event will have zero matched rules", "event_id", evt.ID, "mapping_id", mapObj.ID, "mapping_name", mapObj.Name)
		}
		enabledRules = []rule.Rule{}
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
	for _, r := range enabledRules {
		matched, err := rule.Evaluate(&r, evalFields)
		if err != nil {
			slog.Warn("rule evaluation error", "event_id", evt.ID, "rule_id", r.ID, "rule_name", r.Name, "error", err)
			continue
		}
		slog.Info("rule evaluated", "event_id", evt.ID, "rule_id", r.ID, "rule_name", r.Name, "matched", matched)
		if matched {
			matchedRuleIDs = append(matchedRuleIDs, r.ID)

			// 6. For each matched rule: render template, create deliveries
			created, renderErr := s.processMatchedRule(ctx, tx, eventTx, deliveryTx, evt, src, createdRun, r, evalFields)
			if renderErr != nil {
				// Template render failure — do NOT create empty-content deliveries;
				// fail the event so the error is visible and fixable
				slog.Error("template render failed, skipping deliveries for rule",
					"event_id", evt.ID, "rule_id", r.ID, "rule_name", r.Name, "error", renderErr)
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
		if err := eventTx.FailEventRun(ctx, createdRun.ID, errcode.ProcessorTemplateRender, renderFailure.Error()); err != nil {
			logStateTransition(evt.ID, "fail_event_run", err)
		}
		if err := eventTx.MarkFailed(ctx, evt.ID, errcode.ProcessorTemplateRender, renderFailure.Error()); err != nil {
			logStateTransition(evt.ID, "mark_failed", err)
		}
		slog.Error("event failed: template render error", "event_id", evt.ID, "error", renderFailure)
		return nil
	}
	if err := eventTx.CompleteEventRun(ctx, createdRun.ID, matchedRuleIDs); err != nil {
		logStateTransition(evt.ID, "complete_event_run", err)
	}

	// 8. Mark Event status: deduped if all matched rules were skipped, completed otherwise
	if len(matchedRuleIDs) > 0 && !deliveryCreated {
		if err := eventTx.MarkDeduped(ctx, evt.ID); err != nil {
			logStateTransition(evt.ID, "mark_deduped", err)
		}
		slog.Info("event deduped", "event_id", evt.ID, "matched_rules", len(matchedRuleIDs), "all_deduped", true)
	} else {
		if err := eventTx.MarkProcessed(ctx, evt.ID); err != nil {
			logStateTransition(evt.ID, "mark_processed", err)
		}
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
func (s *Service) processMatchedRule(ctx context.Context, tx pgx.Tx, eventTx *event.Repository, deliveryTx *delivery.Repository, evt *event.Event, src *endpoint.Endpoint, run *event.EventRun, rule rule.Rule, fields map[string]any) (bool, error) {
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
func (s *Service) renderAndCreateDeliveries(ctx context.Context, tx pgx.Tx, deliveryTx *delivery.Repository, evt *event.Event, src *endpoint.Endpoint, run *event.EventRun, rule rule.Rule, fields map[string]any) error {
	if rule.TemplateID == nil || *rule.TemplateID == "" {
		return fmt.Errorf("rule %q has no template configured", rule.Name)
	}

	tmpl, err := s.templateRepo.GetByID(ctx, *rule.TemplateID)
	if err != nil {
		return fmt.Errorf("template %s not found: %w", *rule.TemplateID, err)
	}

	// Build template data: mapped fields at top level + Event metadata
	templateData := make(map[string]any, len(fields)+1)
	maps.Copy(templateData, fields)
	templateData["Event"] = map[string]any{
		"ID":           evt.ID,
		"ReceivedAt":   evt.ReceivedAt.Format(time.RFC3339),
		"EndpointName": src.Name,
	}
	msg, renderErr := template.Render(tmpl, templateData)
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

// formatFieldSummary returns a concise summary of parsed fields for logging.
// Shows each key with its value type and truncated value.
func formatFieldSummary(fields map[string]any) map[string]string {
	summary := make(map[string]string, len(fields))
	for k, v := range fields {
		s := rule.StringifyValue(v)
		if len(s) > 80 {
			s = s[:77] + "..."
		}
		summary[k] = fmt.Sprintf("%s:%s", fmt.Sprintf("%T", v), s)
	}
	return summary
}

// sha256Hash returns the hex-encoded SHA-256 hash of the input string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
