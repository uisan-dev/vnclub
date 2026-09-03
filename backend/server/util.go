package server

import "vnclub/store"

func toUserResponse(u *store.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt}
}
