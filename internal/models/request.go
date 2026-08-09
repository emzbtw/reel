package models

// MediaRequest mirrors the `MediaRequest` schema.
type MediaRequest struct {
	ID        int       `json:"id"`
	Status    int       `json:"status"` // 1 PENDING, 2 APPROVED, 3 DECLINED
	Media     MediaInfo `json:"media"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
	Is4K      bool      `json:"is4k"`
}
