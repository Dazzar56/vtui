# Text segmentation, cell widths and bidirectional text

How vtui turns a Go string into screen cells, and what is still missing.

## The unit of the screen is a grapheme cluster, not a rune

A cell used to hold one rune. That works until the text contains a combining
mark, and then it does not: the mark has no width of its own, so it either
disappeared or, in vtui's case, was replaced by a visible dot, and every
character after it on the line moved. Hindi suffered the most, because almost
every syllable carries one; emoji suffered next, because a modern emoji is
routinely five code points joined by U+200D.

`textseg.go` splits text into extended grapheme clusters (UAX #29, via
`rivo/uniseg`) and gives one cell to one cluster. `CharInfo.Char` holds either
a plain rune, when the cluster is a single one, or an index into a registry of
cluster strings, marked by `CompCharFlag`. This is why `CharInfo.Char` has been
64 bits wide from the start: it mirrors far2l's `COMP_CHAR`. `CellString`,
`CellRunes` and `CellBaseRune` read a cell back.

The registry only ever grows, and each distinct cluster is stored once. A
screenful of text produces a handful of entries; a file viewer scrolling
through prose in a script that composes heavily could produce thousands, which
is still small. It is never cleared, which is a deliberate simplification for
now, see REVIEW.md.

## Widths follow wcwidth, with the emoji exceptions everyone makes

`ClusterWidth` sums the width of the runes of a cluster the way a wcwidth
terminal does. Non spacing marks, enclosing marks and format characters take
no columns; spacing marks (the Devanagari `ा`, for one) take one. Categories
decide before `go-runewidth` does, because `go-runewidth` gives the Devanagari
virama a column and terminals do not.

On top of that, sequences every terminal treats as one double wide glyph are
pinned to two columns: ZWJ sequences, keycaps, skin tone modifiers and pairs
of regional indicators. A cluster carrying U+FE0F is two columns as well,
which is what modern emulators do; set `EmojiPresentationWide` to false on a
strictly wcwidth terminal.

## What to use where

- `StringWidth`, `TruncateString` instead of the `go-runewidth` equivalents.
- `ForEachCluster` / `ForEachClusterAt` to walk text for drawing.
- `AppendCluster` to put a cluster and its fillers into a cell slice.
- `SanitizeCluster` instead of `SanitizeRune`, whenever the surrounding text
  is at hand. `SanitizeRune` remains for callers that only have one rune.

## Staged plan

1. **Done.** The cluster layer, the composite cell registry, cluster aware
   cell producers (`StringToCharInfo`, `FillCharInfo`,
   `FillCharInfoWithSelection`, `StringToCharInfoHighlighted`), the painter and
   the ANSI renderer.
2. Graphical backends draw whole clusters. Today `x11_renderer.go`,
   `wayland_renderer.go` and `gogpu_renderer.go` call `CellBaseRune` and draw
   the base character only, so a composed character loses its marks there while
   keeping the right number of columns. Terminal output is already correct.
3. The remaining `go-runewidth` callers in the widgets, and the three places
   that write text one rune per call: `edit.go`, `multilineedit.go`,
   `vtext.go`.
4. BiDi. The plan is `golang.org/x/text/unicode/bidi`, already an indirect
   dependency: `bidi.Paragraph` for the reordering and `bidi.AppendReverse`
   for mirroring paired brackets. First for read only widgets, where a string
   is laid out once and never edited.
5. BiDi in editable fields: the caret has to move in visual order while the
   buffer stays logical, so `edit.go` needs a logical to visual map. A
   `BidiMode` setting (off / display only / full) keeps this out of the way of
   anyone who does not need it.