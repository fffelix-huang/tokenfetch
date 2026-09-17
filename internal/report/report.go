// Package report aggregates stored usage into the views tokenfetch renders.
// Its JSON form is the contract for external clients (menubar).
package report

import (
	"sort"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/pricing"
	"github.com/fffelix-huang/tokenfetch/internal/store"
	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

// Range is a half-open local time window.
type Range struct {
	Name string    `json:"name"`
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// NewRange builds a named window ending at now: today, week (7 days), month (30 days), all.
func NewRange(name string, now time.Time) (Range, bool) {
	end := now.Add(time.Minute)
	days := map[string]int{"today": 0, "week": 6, "month": 29}
	if name == "all" {
		return Range{Name: name, From: time.Unix(0, 0), To: end}, true
	}
	n, ok := days[name]
	if !ok {
		return Range{}, false
	}
	return Range{Name: name, From: startOfDay(now).AddDate(0, 0, -n), To: end}, true
}

// Row is usage for one value of a dimension.
type Row struct {
	Name     string       `json:"name"`
	Tokens   usage.Tokens `json:"tokens"`
	Total    int64        `json:"total_tokens"`
	CostUSD  float64      `json:"cost_usd"`
	Messages int64        `json:"messages"`
}

// Bucket is usage within one time bucket.
type Bucket struct {
	Total   int64   `json:"total_tokens"`
	CostUSD float64 `json:"cost_usd"`
}

type Day struct {
	Date string `json:"date"` // YYYY-MM-DD local
	Bucket
}

type Report struct {
	Range       Range        `json:"range"`
	Tokens      usage.Tokens `json:"tokens"`
	Total       int64        `json:"total_tokens"`
	CostUSD     float64      `json:"cost_usd"`
	Unpriced    []string     `json:"unpriced_models,omitempty"` // models excluded from cost
	Messages    int64        `json:"messages"`
	Sessions    int64        `json:"sessions"`
	WebSearches int64        `json:"web_searches"`

	Models   []Row `json:"models"`
	Projects []Row `json:"projects"`
	Skills   []Row `json:"skills"`  // excludes usage with no skill
	Plugins  []Row `json:"plugins"` // excludes usage with no plugin
	Agents   []Row `json:"agents"`  // "" = main thread

	Daily     []Day         `json:"daily"`
	HourOfDay [24]Bucket    `json:"hour_of_day"` // local hour
	Heatmap   [7][24]Bucket `json:"heatmap"`     // [weekday Mon=0][local hour]
	PeakHour  int           `json:"peak_hour"`   // -1 if no usage
}

// Build aggregates the store over r in loc.
func Build(st *store.Store, r Range, loc *time.Location) (*Report, error) {
	groups, err := st.Groups(r.From, r.To)
	if err != nil {
		return nil, err
	}
	sessions, err := st.Sessions(r.From, r.To)
	if err != nil {
		return nil, err
	}
	rep := &Report{Range: r, Sessions: sessions, PeakHour: -1}

	dims := map[string]map[string]*Row{}
	add := func(dim, name string, g store.Group, cost float64) {
		m := dims[dim]
		if m == nil {
			m = map[string]*Row{}
			dims[dim] = m
		}
		row := m[name]
		if row == nil {
			row = &Row{Name: name}
			m[name] = row
		}
		row.Tokens.Add(g.Tokens)
		row.Total += g.Total()
		row.CostUSD += cost
		row.Messages += g.Messages
	}
	days := map[string]*Day{}
	unpriced := map[string]bool{}

	for _, g := range groups {
		cost, ok := pricing.Cost(g.Model, g.Speed, g.Geo, g.Tokens, g.WebSearches)
		if !ok {
			unpriced[g.Model] = true
		}
		total := g.Total()
		rep.Tokens.Add(g.Tokens)
		rep.Total += total
		rep.CostUSD += cost
		rep.Messages += g.Messages
		rep.WebSearches += g.WebSearches

		add("model", g.Model, g, cost)
		add("project", g.Project, g, cost)
		add("agent", g.Agent, g, cost)
		if g.Skill != "" {
			add("skill", g.Skill, g, cost)
		}
		if g.Plugin != "" {
			add("plugin", g.Plugin, g, cost)
		}

		t := g.Slot.In(loc)
		h := t.Hour()
		wd := (int(t.Weekday()) + 6) % 7
		rep.HourOfDay[h].Total += total
		rep.HourOfDay[h].CostUSD += cost
		rep.Heatmap[wd][h].Total += total
		rep.Heatmap[wd][h].CostUSD += cost

		key := t.Format("2006-01-02")
		d := days[key]
		if d == nil {
			d = &Day{Date: key}
			days[key] = d
		}
		d.Total += total
		d.CostUSD += cost
	}

	rep.Models = sorted(dims["model"])
	rep.Projects = sorted(dims["project"])
	rep.Skills = sorted(dims["skill"])
	rep.Plugins = sorted(dims["plugin"])
	rep.Agents = sorted(dims["agent"])

	for _, d := range days {
		rep.Daily = append(rep.Daily, *d)
	}
	sort.Slice(rep.Daily, func(i, j int) bool { return rep.Daily[i].Date < rep.Daily[j].Date })

	for m := range unpriced {
		rep.Unpriced = append(rep.Unpriced, m)
	}
	sort.Strings(rep.Unpriced)

	var peak int64
	for h, b := range rep.HourOfDay {
		if b.Total > peak {
			peak, rep.PeakHour = b.Total, h
		}
	}
	return rep, nil
}

func sorted(m map[string]*Row) []Row {
	out := make([]Row, 0, len(m))
	for _, r := range m {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Name < out[j].Name
	})
	return out
}
