package research

import "reflect"

// reconcile is deliberately conservative, scoped to one source observation.
// The caller must have resolved source lineage before calling it. Evidence is
// explicit test input, not authority inferred from stable IDs or timestamps.
// There is no production correction evidence provider in this milestone.
func reconcile(old, next *Observation, verifiedCorrections map[string]bool) (*Observation, string) {
	if next == nil || !next.Complete {
		return old, "incomplete"
	}
	if old == nil {
		return next, "accepted"
	}
	if old.Scope != next.Scope {
		return old, "unresolved-scope"
	}
	if old.Schema != next.Schema || old.Policy != next.Policy {
		return old, "unresolved-migration"
	}
	if len(next.Diagnostics) > 0 && !reflect.DeepEqual(old, next) {
		return old, "unresolved-fields"
	}
	for key, previous := range old.Items {
		current, present := next.Items[key]
		if !present {
			return old, "unresolved-record-loss"
		}
		if !reflect.DeepEqual(previous, current) && !verifiedCorrections[key] {
			return old, "unresolved-correction"
		}
	}
	// This prototype accepts append-only growth ONLY within the same validated
	// native identity space. Renumbered replacements that remove old IDs fail the
	// check above. Projection/event overlap without removals still needs upstream
	// semantics; do not treat this function as a generic source policy.
	return next, "accepted"
}
