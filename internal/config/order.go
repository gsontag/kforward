package config

import (
	"cmp"
	"slices"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Sort puts forwards in canonical order: by group, forwards without group
// last, then by name, alphabetically for a human. The order of the file
// does not matter; the UUID only breaks ties between identical names.
func Sort(forwards []Forward) {
	// A collator keeps state between comparisons: one per call, never shared
	c := collate.New(language.Und, collate.IgnoreCase)
	slices.SortFunc(forwards, func(a, b Forward) int {
		return cmp.Or(
			compareGroups(c, a.Group, b.Group),
			c.CompareString(a.Name, b.Name),
			cmp.Compare(a.UUID, b.UUID),
		)
	})
}

func compareGroups(c *collate.Collator, a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	default:
		// Equal for the collator is not enough: "Web" and "web" would mix
		return cmp.Or(c.CompareString(a, b), cmp.Compare(a, b))
	}
}
