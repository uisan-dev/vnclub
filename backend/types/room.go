package types

type Room struct {
	ID          uint     `json:"id"`
	RoomTitle   string   `json:"room_title"`
	MemberCount int      `json:"member_count"`
	Tags        []string `json:"tags"`
	InviteOnly  bool     `json:"invite_only"`
}
