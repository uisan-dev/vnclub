package types

import "time"

type Room struct {
	ID          uint      `json:"id"`
	Title       string    `json:"room_title"`
	VNID        string    `json:"vn_id"`
	VNTitle     string    `json:"vn_title"`
	MemberCount int       `json:"member_count"`
	Tags        []string  `json:"tags"`
	InviteOnly  bool      `json:"invite_only"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
