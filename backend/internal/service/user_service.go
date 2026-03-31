package service

import (
	"errors"
)

var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	users UserStore
}

func NewUserService(users UserStore) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetProfile(userID string) (*UserProfile, error) {
	user, err := s.users.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	profile := ToUserProfile(user)
	return &profile, nil
}
