package obsidian

import "testing"

func TestParseMarker(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want Marker
	}{
		{"space is unsynced", ' ', Unsynced},
		{"clapper is requested", '🎬', Requested},
		{"down arrow is downloading", '↓', Downloading},
		{"check is available", '✓', Available},
		{"cross is failed", '✗', Failed},
		{"lowercase x is done", 'x', Done},
		{"uppercase X is done", 'X', Done},
		{"dash is foreign (Tasks plugin cancelled)", '-', Foreign},
		{"slash is foreign (Tasks plugin in progress)", '/', Foreign},
		{"gt is foreign (Tasks plugin forwarded)", '>', Foreign},
		{"unknown rune is foreign", '?', Foreign},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMarker(tt.r); got != tt.want {
				t.Errorf("ParseMarker(%q) = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestMarker_Writable(t *testing.T) {
	tests := []struct {
		m    Marker
		want bool
	}{
		{Unsynced, true},
		{Requested, true},
		{Downloading, true},
		{Available, true},
		{Failed, true},
		{Done, false},
		{Foreign, false},
	}
	for _, tt := range tests {
		if got := tt.m.Writable(); got != tt.want {
			t.Errorf("%v.Writable() = %v, want %v", tt.m, got, tt.want)
		}
	}
}

func TestMarker_RuneRoundTrip(t *testing.T) {
	// Every writable marker must survive ParseMarker(m.Rune()) == m, since
	// WriteEdits relies on Rune() to produce bytes that later parse back to
	// the same Marker.
	writable := []Marker{Unsynced, Requested, Downloading, Available, Failed}
	for _, m := range writable {
		r := m.Rune()
		if got := ParseMarker(r); got != m {
			t.Errorf("ParseMarker(%v.Rune()) = %v, want %v", m, got, m)
		}
	}
}

func TestMarker_RunePanicsOnNonWritable(t *testing.T) {
	for _, m := range []Marker{Done, Foreign} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%v.Rune() did not panic", m)
				}
			}()
			m.Rune()
		}()
	}
}

func TestParseTitle(t *testing.T) {
	tests := []struct {
		name string
		text string
		want TitleInfo
	}{
		{
			name: "plain title",
			text: "Arrival",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "wikilink",
			text: "[[Arrival]]",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "aliased wikilink uses display text after the pipe",
			text: "[[tt2543164|Arrival]]",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "markdown link",
			text: "[Arrival](https://www.themoviedb.org/movie/329865)",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			// Writing the year outside the link is the natural way to hint a
			// year at reel without repointing the wikilink: [[Alien (1979)]]
			// targets a different note than [[Alien]].
			name: "year hint outside a wikilink",
			text: "[[Alien]] (1979)",
			want: TitleInfo{Title: "Alien", Year: "1979"},
		},
		{
			name: "year hint inside a wikilink",
			text: "[[Alien (1979)]]",
			want: TitleInfo{Title: "Alien", Year: "1979"},
		},
		{
			name: "year hint outside a markdown link",
			text: "[Alien](https://example.com/alien) (1979)",
			want: TitleInfo{Title: "Alien", Year: "1979"},
		},
		{
			// Unwrapping here would discard "and [[Heat]]" silently. Better
			// to leave it whole and let it fail visibly as an unresolvable
			// title.
			name: "two wikilinks on one line are not unwrapped",
			text: "[[Alien]] and [[Heat]]",
			want: TitleInfo{Title: "[[Alien]] and [[Heat]]"},
		},
		{
			name: "trailing hashtag",
			text: "Arrival #film",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "multiple trailing hashtags",
			text: "Arrival #film #to-watch",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "nested trailing hashtag",
			text: "Arrival #a/b",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "trailing block id",
			text: "Arrival ^abc123",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "trailing hashtag and block id",
			text: "Arrival #film ^abc123",
			want: TitleInfo{Title: "Arrival"},
		},
		{
			name: "trailing year",
			text: "Alien (1979)",
			want: TitleInfo{Title: "Alien", Year: "1979"},
		},
		{
			name: "non-trailing parenthesized year does not count",
			text: "Se7en (1995) (dir cut)",
			want: TitleInfo{Title: "Se7en (1995) (dir cut)"},
		},
		{
			name: "year out of accepted range is left alone",
			text: "Report (1500)",
			want: TitleInfo{Title: "Report (1500)"},
		},
		{
			name: "ignore token",
			text: "Arrival %%reel:ignore%%",
			want: TitleInfo{Title: "Arrival", Ignored: true},
		},
		{
			name: "ignore token case-insensitive",
			text: "Arrival %%REEL:IGNORE%%",
			want: TitleInfo{Title: "Arrival", Ignored: true},
		},
		{
			name: "ignore token combined with year and tag",
			text: "Alien (1979) #film %%reel:ignore%%",
			want: TitleInfo{Title: "Alien", Year: "1979", Ignored: true},
		},
		{
			name: "wikilink with trailing tag",
			text: "[[Alien (1979)]] #film",
			want: TitleInfo{Title: "Alien", Year: "1979"},
		},
		{
			name: "whitespace trimmed",
			text: "  Arrival  ",
			want: TitleInfo{Title: "Arrival"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTitle(tt.text)
			if got != tt.want {
				t.Errorf("ParseTitle(%q) = %+v, want %+v", tt.text, got, tt.want)
			}
		})
	}
}
