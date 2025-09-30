package fmcore

// MergeMode defines how a patch applies to existing front matter.
type MergeMode int

const (
	MergeDeep MergeMode = iota // deep merge maps; arrays follow strategy
	MergeReplace               // replace entire target keys
	MergeSetIfMissing          // only set keys absent in base
)

// ArrayMergeStrategy controls how arrays are merged when both old and new are slices under Deep mode.
type ArrayMergeStrategy int

const (
	ArrayReplace ArrayMergeStrategy = iota
	ArrayUnion
	ArrayAppend
)

// FrontMatterPatch represents a unit of front matter changes from a transformer.
type FrontMatterPatch struct {
	Source        string
	Mode          MergeMode
	Priority      int                // higher applied later
	Data          map[string]any     // patch data
	ArrayStrategy ArrayMergeStrategy // optional override for all arrays in this patch (0 value = replace)
}
