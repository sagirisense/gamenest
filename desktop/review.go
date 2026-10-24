package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"path/filepath"
	"sort"
)

type MetadataReview struct {
	Pending bool   `json:"pending"`
	Reason  string `json:"reason"`
	Count   int    `json:"count"`
}

func metadataCandidates(m map[string]any) []Candidate {
	b, _ := json.Marshal(m["candidates"])
	var cs []Candidate
	_ = json.Unmarshal(b, &cs)
	return cs
}

// Persist a decision for this candidate set, so repeated scans do not nag again.
func candidateSignature(cs []Candidate) string {
	keys := make([]string, 0, len(cs))
	for _, c := range cs {
		keys = append(keys, c.Key)
	}
	sort.Strings(keys)
	b, _ := json.Marshal(keys)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func candidateReviewReason(cs []Candidate) string {
	if len(cs) == 0 {
		return ""
	}
	best := cs[0]
	for _, c := range cs {
		if c.Score > best.Score {
			best = c
		}
	}
	for _, c := range cs {
		if c.Key == best.Key {
			continue
		}
		// A shared title across stores is not proof that two records are one work.
		if math.Abs(best.Score-c.Score) < 0.000001 {
			return "匹配率相同"
		}
	}
	for _, c := range cs {
		if c.Key != best.Key && best.Score-c.Score < 0.08 {
			return "匹配率接近"
		}
	}
	if best.Score < 0.92 {
		return "需要核对作品"
	}
	return ""
}

func metadataReview(m map[string]any) MetadataReview {
	cs := metadataCandidates(m)
	r := MetadataReview{Count: len(cs)}
	if len(cs) == 0 {
		return r
	}
	if str(m, "review_resolved") == candidateSignature(cs) {
		return r
	}
	if value, ok := m["review_pending"].(bool); ok {
		r.Pending = value
		r.Reason = str(m, "review_reason")
		return r
	}
	// Older libraries already contain candidates, but no explicit review state.
	if str(m, "search_provider") == "all" && str(m, "source_url") == "" {
		r.Reason = candidateReviewReason(cs)
		r.Pending = r.Reason != ""
	}
	return r
}

func (a *App) KeepMetadata(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.game(id); err != nil {
		return err
	}
	old := a.meta[id]
	next := map[string]any{}
	for k, v := range old {
		next[k] = v
	}
	next["review_resolved"] = candidateSignature(metadataCandidates(old))
	next["review_pending"] = false
	next["review_decision"] = "kept"
	a.meta[id] = next
	if err := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta); err != nil {
		a.meta[id] = old
		return err
	}
	return nil
}
