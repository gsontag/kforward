package config

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// names lists the forwards as "group/name", or "/name" without group.
func names(forwards []Forward) string {
	parts := make([]string, 0, len(forwards))
	for _, f := range forwards {
		parts = append(parts, f.Group+"/"+f.Name)
	}
	return strings.Join(parts, " ")
}

func TestSort(t *testing.T) {
	tests := []struct {
		name     string
		forwards []Forward
		want     string
	}{
		{"groups first, ungrouped last", []Forward{
			{UUID: "1", Name: "zeta"},
			{UUID: "2", Name: "web", Group: "Web"},
			{UUID: "3", Name: "db", Group: "Bases"},
		}, "Bases/db Web/web /zeta"},
		{"names within a group", []Forward{
			{UUID: "1", Name: "mariadb", Group: "Bases"},
			{UUID: "2", Name: "etcd", Group: "Bases"},
		}, "Bases/etcd Bases/mariadb"},
		// Byte order would put every capital first
		{"case ignored", []Forward{
			{UUID: "1", Name: "Zeta"},
			{UUID: "2", Name: "alpha"},
		}, "/alpha /Zeta"},
		// Byte order would put an accented capital after "z"
		{"accents", []Forward{
			{UUID: "1", Name: "mariadb"},
			{UUID: "2", Name: "Élasticsearch"},
			{UUID: "3", Name: "dup"},
		}, "/dup /Élasticsearch /mariadb"},
		{"accented groups", []Forward{
			{UUID: "1", Name: "a", Group: "Zones"},
			{UUID: "2", Name: "a", Group: "Éditions"},
		}, "Éditions/a Zones/a"},
		// Equal for the collator, still two distinct and contiguous groups
		{"groups differing by case", []Forward{
			{UUID: "1", Name: "grafana", Group: "observabilité"},
			{UUID: "2", Name: "alertmanager", Group: "Observabilité"},
			{UUID: "3", Name: "loki", Group: "observabilité"},
		}, "Observabilité/alertmanager observabilité/grafana observabilité/loki"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Sort(tt.forwards)
			check(t, "order", names(tt.forwards), tt.want)
		})
	}
}

func TestSortTieBrokenByUUID(t *testing.T) {
	forwards := []Forward{{UUID: "b", Name: "dup"}, {UUID: "a", Name: "dup"}}
	Sort(forwards)
	check(t, "uuids", forwards[0].UUID+forwards[1].UUID, "ab")
}

// The order of the file must not matter: every shuffle sorts the same.
func TestSortIndependentOfInputOrder(t *testing.T) {
	reference := []Forward{
		{UUID: "1", Name: "grafana", Group: "Observabilité"},
		{UUID: "2", Name: "Élasticsearch", Group: "Bases"},
		{UUID: "3", Name: "dup", Group: "Bases"},
		{UUID: "4", Name: "dup", Group: "Bases"},
		{UUID: "5", Name: "keycloak"},
		{UUID: "6", Name: "Alpha"},
	}
	Sort(reference)

	// Fixed seed: a failure can be replayed
	rng := rand.New(rand.NewPCG(1, 2))
	for range 50 {
		shuffled := slices.Clone(reference)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		Sort(shuffled)
		if !slices.Equal(uuids(shuffled), uuids(reference)) {
			t.Fatalf("got %v, want %v", uuids(shuffled), uuids(reference))
		}
	}
}

func uuids(forwards []Forward) []string {
	ids := make([]string, 0, len(forwards))
	for _, f := range forwards {
		ids = append(ids, f.UUID)
	}
	return ids
}
