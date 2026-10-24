package main

import (
	"sort"
	"unicode"
)

func collectEnrichmentCandidates(g Game, source, query string) ([]Candidate, []string) {
	all := []Candidate{}
	failures := []string{}
	add := func(cs []Candidate, err error) {
		all = append(all, cs...)
		if err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(localRJIDs(g, source)) > 0 {
		add(localDLSiteLookup(g, source))
	}
	appid := parseAppID(source)
	if appid == "" {
		appid = detectAppID(g)
	}
	if appid != "" && appid != "480" {
		add(steamLookup(appid))
	}
	// Every source is queried before selecting a match, even after an exact hit.
	add(steamLookup(query))
	add(dlsiteLookup(query))
	add(vndbLookup(query))
	return all, failures
}

func titleSimilarity(a, b string) float64 {
	a = norm(backgroundName(a))
	b = norm(backgroundName(b))
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	digits := func(s string) string {
		out := []rune{}
		for _, r := range s {
			if unicode.IsDigit(r) {
				out = append(out, r)
			}
		}
		return string(out)
	}
	if digits(a) != digits(b) {
		return 0
	} // Preserve sequel numbers.
	x, y := []rune(a), []rune(b)
	previous := make([]int, len(y)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, rx := range x {
		row := make([]int, len(y)+1)
		row[0] = i + 1
		for j, ry := range y {
			cost := 0
			if rx != ry {
				cost = 1
			}
			row[j+1] = min(row[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = row
	}
	return 1 - float64(previous[len(y)])/float64(max(len(x), len(y)))
}

func rankEnrichmentCandidates(all []Candidate, query string, g Game, source string) ([]Candidate, Candidate, bool) {
	appid := parseAppID(source)
	if appid == "" {
		appid = detectAppID(g)
	}
	local := localRJIDs(g, source)
	byKey := map[string]Candidate{}
	for _, c := range all {
		score := titleSimilarity(query, c.Name)
		for _, alias := range c.Aliases {
			score = max(score, titleSimilarity(query, alias))
		}
		if appid != "" && appid != "480" && c.Key == "steam:"+appid {
			score = 1.25
		}
		if len(local) == 1 && c.Key == "dlsite:"+local[0] {
			score = 1.25
		}
		c.Score = score
		c.Exact = score >= 1
		if previous, ok := byKey[c.Key]; !ok || c.Score > previous.Score || previous.Image == "" && c.Image != "" {
			byKey[c.Key] = c
		}
	}
	ranked := []Candidate{}
	for _, c := range byKey {
		ranked = append(ranked, c)
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if (ranked[i].Image != "") != (ranked[j].Image != "") {
			return ranked[i].Image != ""
		}
		return ranked[i].Key < ranked[j].Key
	})
	if len(ranked) == 0 {
		return ranked, Candidate{}, false
	}
	best := ranked[0]
	confident := best.Score >= 0.92
	if len(local) > 1 {
		confident = false
	}
	for _, other := range ranked[1:] {
		if best.Key != other.Key && best.Score-other.Score < 0.08 {
			confident = false
		}
	}
	return ranked, best, confident
}

func sameCandidateWork(a, b Candidate) bool {
	for _, x := range append([]string{a.Name}, a.Aliases...) {
		for _, y := range append([]string{b.Name}, b.Aliases...) {
			nx, ny := norm(backgroundName(x)), norm(backgroundName(y))
			if nx != "" && nx == ny {
				return true
			}
		}
	}
	return false
}
