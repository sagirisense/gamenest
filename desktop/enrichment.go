package main

import (
	"path/filepath"
	"strings"
)

type EnrichmentResult struct {
	Name       bool     `json:"name"`
	Cover      bool     `json:"cover"`
	Background bool     `json:"background"`
	Review     bool     `json:"review"`
	Errors     []string `json:"errors"`
}

// EnrichGame only fills missing fields; user edits and existing artwork win.
func (a *App) EnrichGame(id string) (EnrichmentResult, error) {
	result := EnrichmentResult{Errors: []string{}}
	a.mu.Lock()
	g, err := a.game(id)
	name := str(a.meta[id], "name")
	source := str(a.meta[id], "source_url")
	coverPath := a.imagePath(g)
	needCover := !exists(coverPath) || strings.Contains(coverPath, string(filepath.Separator)+"icons"+string(filepath.Separator))
	needName := name == ""
	needBackground := a.heroPath(g) == ""
	pendingReview := metadataReview(a.meta[id]).Pending
	a.mu.Unlock()
	if err != nil {
		return result, err
	}
	if pendingReview && !needName && !needCover {
		result.Review = true
		return result, nil
	}
	if needCover {
		ensureGameIcons(a.dir, []Game{g})
	}
	if needName || needCover {
		query := backgroundName(name)
		if query == "" {
			query = backgroundName(g.Name)
		}
		all, lookupErrors := collectEnrichmentCandidates(g, source, query)
		result.Errors = append(result.Errors, lookupErrors...)
		ranked, candidate, matched := rankEnrichmentCandidates(all, query, g, source)
		if len(ranked) == 0 && pendingReview {
			// A failed/empty retry must not discard a decision still awaiting review.
			result.Review = true
			return result, nil
		}
		if _, e := a.storeMetadataCandidates(id, query, "all", ranked); e != nil {
			return result, e
		}
		a.mu.Lock()
		kept := str(a.meta[id], "review_decision") == "kept" && str(a.meta[id], "review_resolved") == candidateSignature(ranked)
		a.mu.Unlock()
		if kept {
			return result, nil
		}
		result.Review = len(ranked) > 0 && !matched
		if result.Review {
			a.mu.Lock()
			m := a.meta[id]
			m["review_pending"] = true
			if str(m, "review_reason") == "" {
				m["review_reason"] = "本地编号存在冲突"
			}
			result.Review = metadataReview(m).Pending
			e := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta)
			a.mu.Unlock()
			if e != nil {
				return result, e
			}
			// Do not choose a background for an unresolved work identity either.
			return result, nil
		}
		if matched {

			image := ""
			coverCandidate := candidate
			if needCover {
				for _, option := range ranked {
					if option.Image == "" || option.Key != candidate.Key && !sameCandidateWork(candidate, option) {
						continue
					}
					b, e := fetchCandidateCover(option)
					if e == nil {
						image, e = a.saveImage(id, b)
					}
					if e != nil {
						result.Errors = append(result.Errors, e.Error())
						continue
					}
					coverCandidate = option
					break
				}
			}
			a.mu.Lock()
			old := a.meta[id]
			next := map[string]any{}
			for k, v := range old {
				next[k] = v
			}
			if needName && str(old, "name") == "" && candidate.Name != "" {
				next["name"] = candidate.Name
				result.Name = true
			}
			if image != "" && a.imagePath(g) == coverPath {
				next["image"] = image
				next["cover_provider"] = coverCandidate.Provider
				next["cover_source_url"] = coverCandidate.URL
				result.Cover = true
			}
			if str(old, "source_url") == "" {
				next["source_url"] = candidate.URL
				next["provider"] = candidate.Provider
			}
			a.meta[id] = next
			e := writeJSON(filepath.Join(a.dir, "metadata.json"), a.meta)
			if e != nil {
				a.meta[id] = old
				result.Name = false
				result.Cover = false
				result.Errors = append(result.Errors, e.Error())
			}
			a.mu.Unlock()
		}

	}
	if needBackground {
		bg, e := a.FindBackground(id, "")
		if e != nil {
			result.Errors = append(result.Errors, e.Error())
		} else {
			result.Background = bg.Status == "found"
			result.Review = result.Review || bg.Status == "review"
			if bg.Status == "failed" {
				result.Errors = append(result.Errors, bg.Message)
			}
		}
	}
	return result, nil
}
