package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type capacityResult struct {
	Shape                                                                           string
	Rows                                                                            int
	AccountingPayloadBytes                                                          int64
	StageNS, PublishNS                                                              int64
	ReplacementStageNS, ReplacementPublishNS, ExistingArchiveBytesBeforeReplacement int64
	Resources                                                                       resourceResult
}

// Resource probe near the 64 MiB payload cap, deliberately distinct from the
// ordinary/huge latency shapes. The 15 s diagnostic timeout is NOT a fallback
// or acceptance measurement for the 2 s source-reader deadline.
func TestHistoryGateCapacity(t *testing.T) {
	if !*historyGateC {
		t.Skip("opt-in synthetic capacity probe")
	}
	for _, distinct := range []bool{false, true} {
		name := "shared-labels"
		if distinct {
			name = "distinct-models"
		}
		t.Run(name, func(t *testing.T) { measureGateCapacity(t, name, distinct) })
	}
}

func measureGateCapacity(t *testing.T, name string, distinct bool) {
	path, writer := fixture(t)
	requireSQL(t, writer, `INSERT INTO session VALUES('cap','Synthetic','/synthetic',1,1,0,NULL)`)
	data := payload(false, strings.Repeat("p", 3072), strings.Repeat("m", 128), strings.Repeat("v", 128), .125)
	requireSQL(t, writer, `INSERT INTO message VALUES('cap00000000','cap',1,1,?)`, data)
	source := openReader(t, path)
	o, err := extract(context.Background(), source, extractOptions{Session: "cap"})
	if err != nil {
		t.Fatal(err)
	}
	u := o.Items["cap\x00message\x00cap00000000"]
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	size := len(raw) + len(usageKey(u))
	count := (64 << 20) / size
	requireSQL(t, writer, `WITH RECURSIVE ids(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM ids WHERE n<?) INSERT INTO message SELECT printf('cap%08d',n),'cap',1,1,? FROM ids`, count-1, data)
	if distinct {
		requireSQL(t, writer, `UPDATE message SET data=json_set(data,'$.modelID',substr(?,1,120)||substr(id,4))`, strings.Repeat("m", 128))
	}
	requireSQL(t, writer, `PRAGMA wal_checkpoint(TRUNCATE)`)
	archive, archivePath := pipelineArchive(t)
	stage := newStage(t)
	sampler := sampleResources(archivePath)
	sampler.add(stage.path)
	ctx := context.Background()
	capture := func(db *sql.DB) error {
		readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		_, e := stageSession(readCtx, source, db, extractOptions{Session: "cap"})
		return e
	}
	at := time.Now()
	err = capture(stage.db)
	stageNS := time.Since(at).Nanoseconds()
	if err != nil {
		_ = sampler.finish()
		t.Fatal(err)
	}
	at = time.Now()
	_, err = publishSession(ctx, archive, "cap", nil, stage.db, publishOptions{})
	pubNS := time.Since(at).Nanoseconds()

	if err != nil {
		_ = sampler.finish()
		t.Fatal(err)
	}
	var rows int
	if err = archive.QueryRow(`SELECT count(*) FROM contributions`).Scan(&rows); err != nil || rows != count {
		t.Fatal(rows, count, err)
	}
	if acceptedCost(t, archive) != float64(count)*.125 {
		t.Fatal("capacity oracle mismatch")
	}
	// Repeat against populated accounting, with explicit per-ID synthetic authority.
	_ = stage.db.Close()
	_ = os.Remove(stage.path)
	existing := optionalBytes(archivePath)
	requireSQL(t, writer, `UPDATE message SET data=json_set(data,'$.cost',0)`)
	replacement := newStage(t)
	sampler.add(replacement.path)
	at = time.Now()
	err = capture(replacement.db)
	replacementStageNS := time.Since(at).Nanoseconds()
	if err != nil {
		_ = sampler.finish()
		t.Fatal(err)
	}
	authority := make(map[string]bool, count)
	for i := range count {
		authority[fmt.Sprintf("cap\x00message\x00cap%08d", i)] = true
	}
	at = time.Now()
	_, err = publishSession(ctx, archive, "cap", nil, replacement.db, publishOptions{Expected: 1, Corrections: authority})
	replacementPubNS := time.Since(at).Nanoseconds()
	if err != nil {
		_ = sampler.finish()
		t.Fatal(err)
	}
	if acceptedCost(t, archive) != 0 {
		_ = sampler.finish()
		t.Fatal("capacity replacement oracle mismatch")
	}
	resources := sampler.finish()
	_ = replacement.db.Close()
	_ = os.Remove(replacement.path)
	r := capacityResult{Shape: name, Rows: count, AccountingPayloadBytes: int64(count * size), StageNS: stageNS, PublishNS: pubNS, ReplacementStageNS: replacementStageNS, ReplacementPublishNS: replacementPubNS, ExistingArchiveBytesBeforeReplacement: existing, Resources: resources}
	encoded, _ := json.Marshal(r)
	fmt.Printf("HISTORY_CAPACITY %s\n", encoded)
	// One extra eligible record must fail the payload cap without a seal, and
	// must never invalidate the already published near-cap generation.
	requireSQL(t, writer, `UPDATE message SET data=json_set(data,'$.cost',0.125)`)
	requireSQL(t, writer, `INSERT INTO message VALUES('cap99999999','cap',1,1,?)`, data)
	over := newStage(t)
	if err = capture(over.db); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatal("payload cap did not reject extra row", err)
	}
	assertUnsealed(t, over.db)
	if acceptedCost(t, archive) != 0 {
		t.Fatal("cap rejection changed accepted totals")
	}
}
