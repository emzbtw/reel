package models

// MediaRequest mirrors the `MediaRequest` schema.
type MediaRequest struct {
	ID        int             `json:"id"`
	Status    int             `json:"status"` // 1 PENDING, 2 APPROVED, 3 DECLINED, 4 FAILED, 5 COMPLETED
	Media     MediaInfo       `json:"media"`
	Type      MediaType       `json:"type"` // "movie" or "tv" — also mirrored at media.mediaType
	Seasons   []RequestSeason `json:"seasons"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	Is4K      bool            `json:"is4k"`
}

// RequestSeason is one element of MediaRequest.Seasons: a TV request's
// per-season availability. Movie requests always have an empty Seasons
// slice.
type RequestSeason struct {
	ID           int         `json:"id"`
	SeasonNumber int         `json:"seasonNumber"`
	Status       MediaStatus `json:"status"` // same enum as MediaInfo.Status
}
